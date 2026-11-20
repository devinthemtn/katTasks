// Package script runs the user's Lua scripts from the scripts folder next to
// config.toml.
//
// A script sees a global table `kat`:
//
//	kat.input        the task being typed, without the /name word ("" from :)
//	kat.task         the task under the cursor (or being edited), or nil
//	kat.tasks()      every task in todo.txt
//	kat.date(s)      "fri", "+3d", "tomorrow"… as YYYY-MM-DD, or nil
//	kat.today        today as YYYY-MM-DD
//	kat.add(line)    add a task
//	kat.update(line) replace the task under the cursor (from : only)
//	kat.status(msg)  show a message in the footer (print does the same)
//	kat.choose(prompt, {opts})  ask the user to pick one (inline only);
//	                 returns the chosen string, or nil if cancelled
//	kat.replace(line) replace the whole task being typed (inline only)
//
// A task is a table with line, text, done, priority, due, projects,
// contexts and tags. Inline, the script's return value replaces the /name
// word; from :, a returned string is shown as the status message.
package script

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// Timeout stops a script that runs too long.
const Timeout = 2 * time.Second

// Script is one .lua file in the scripts folder.
type Script struct {
	Name string // file name without .lua
	Desc string // first "-- " comment line, if any
	Path string
}

// Dir is the scripts folder for a config folder.
func Dir(configDir string) string { return filepath.Join(configDir, "scripts") }

// List returns the scripts in dir, sorted by name. A missing folder has none.
func List(dir string) []Script {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Script
	for _, e := range ents {
		name, ok := strings.CutSuffix(e.Name(), ".lua")
		if !ok || e.IsDir() || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		out = append(out, Script{Name: name, Desc: description(p), Path: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// description is the text of the first line if it is a "--" comment.
func description(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return ""
	}
	line := strings.TrimSpace(sc.Text())
	if d, ok := strings.CutPrefix(line, "--"); ok {
		return strings.TrimSpace(d)
	}
	return ""
}

// Env is what a script can see.
type Env struct {
	Inline  bool       // run from the task input with /name
	Input   string     // inline: the typed text without the /name word
	Current *todo.Task // task under the cursor or being edited; may be nil
	Tasks   []*todo.Task
	Now     time.Time
	// Answers are the kat.choose picks made so far, in call order. A
	// script that calls kat.choose once more than there are answers stops,
	// and Result.Choice says what to ask; the UI runs it again with the
	// pick appended ("" for cancel, which kat.choose returns as nil).
	Answers []string
}

// Choice is a question a script asked with kat.choose.
type Choice struct {
	Prompt  string
	Options []string
}

// Result is what a script asked for.
type Result struct {
	Output    string // the returned value, as text
	HasOutput bool
	Adds      []string // kat.add lines, in order
	Update    *string  // kat.update line
	Replace   *string  // kat.replace line
	Choice    *Choice  // set when the script is waiting on kat.choose
	Status    string   // kat.status / print, joined
}

// Run runs s with env.
func Run(s Script, env Env) (Result, error) {
	var res Result
	L := lua.NewState()
	defer L.Close()
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	L.SetContext(ctx)

	var status []string
	addStatus := func(L *lua.LState) int {
		parts := make([]string, L.GetTop())
		for i := range parts {
			parts[i] = L.ToStringMeta(L.Get(i + 1)).String()
		}
		status = append(status, strings.Join(parts, " "))
		return 0
	}
	L.SetGlobal("print", L.NewFunction(addStatus)) // stdout belongs to the UI

	kat := L.NewTable()
	L.SetField(kat, "input", lua.LString(env.Input))
	L.SetField(kat, "today", lua.LString(env.Now.Format(todo.DateLayout)))
	if env.Current != nil {
		L.SetField(kat, "task", taskTable(L, env.Current))
	}
	L.SetField(kat, "tasks", L.NewFunction(func(L *lua.LState) int {
		t := L.CreateTable(len(env.Tasks), 0)
		for _, task := range env.Tasks {
			t.Append(taskTable(L, task))
		}
		L.Push(t)
		return 1
	}))
	L.SetField(kat, "date", L.NewFunction(func(L *lua.LState) int {
		d, ok := todo.ResolveDate(L.CheckString(1), env.Now)
		if !ok {
			if p, err := time.Parse(todo.DateLayout, L.CheckString(1)); err == nil {
				d, ok = p, true
			}
		}
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(d.Format(todo.DateLayout)))
		return 1
	}))
	L.SetField(kat, "add", L.NewFunction(func(L *lua.LState) int {
		if line := strings.TrimSpace(L.CheckString(1)); line != "" {
			res.Adds = append(res.Adds, line)
		}
		return 0
	}))
	L.SetField(kat, "update", L.NewFunction(func(L *lua.LState) int {
		if env.Inline {
			L.RaiseError("kat.update only works when the script is run from :")
		}
		if env.Current == nil {
			L.RaiseError("kat.update: no task under the cursor")
		}
		line := strings.TrimSpace(L.CheckString(1))
		if line == "" {
			L.RaiseError("kat.update: empty task")
		}
		res.Update = &line
		return 0
	}))
	L.SetField(kat, "status", L.NewFunction(addStatus))
	asked := 0
	var choice *Choice
	L.SetField(kat, "choose", L.NewFunction(func(L *lua.LState) int {
		if !env.Inline {
			L.RaiseError("kat.choose only works in a /name script")
		}
		prompt := L.CheckString(1)
		opts := L.CheckTable(2)
		var list []string
		opts.ForEach(func(_, v lua.LValue) { list = append(list, L.ToStringMeta(v).String()) })
		if len(list) == 0 {
			L.RaiseError("kat.choose: no options")
		}
		if asked < len(env.Answers) {
			a := env.Answers[asked]
			asked++
			if a == "" {
				L.Push(lua.LNil)
			} else {
				L.Push(lua.LString(a))
			}
			return 1
		}
		choice = &Choice{Prompt: prompt, Options: list}
		L.RaiseError("%s", errChoose)
		return 0
	}))
	L.SetField(kat, "replace", L.NewFunction(func(L *lua.LState) int {
		if !env.Inline {
			L.RaiseError("kat.replace only works in a /name script")
		}
		line := strings.TrimSpace(L.CheckString(1))
		res.Replace = &line
		return 0
	}))
	L.SetGlobal("kat", kat)

	if err := L.DoFile(s.Path); err != nil {
		if choice != nil {
			// Waiting on the user; nothing else the script did counts yet.
			return Result{Choice: choice}, nil
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, fmt.Errorf("%s: stopped after %s", s.Name, Timeout)
		}
		return Result{}, fmt.Errorf("%s: %s", s.Name, firstLine(err.Error()))
	}
	if choice != nil { // a pcall swallowed the stop
		return Result{Choice: choice}, nil
	}
	if L.GetTop() > 0 {
		switch v := L.Get(1); v.Type() {
		case lua.LTNil:
		case lua.LTBool:
			if v == lua.LTrue {
				res.Output, res.HasOutput = "true", true
			}
		default:
			res.Output, res.HasOutput = L.ToStringMeta(v).String(), true
		}
	}
	res.Status = strings.Join(status, " · ")
	return res, nil
}

// errChoose stops a script at a kat.choose that has no answer yet.
const errChoose = "kat.choose: waiting for a choice"

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}

func taskTable(L *lua.LState, t *todo.Task) *lua.LTable {
	tb := L.NewTable()
	L.SetField(tb, "line", lua.LString(t.String()))
	L.SetField(tb, "text", lua.LString(t.Text))
	L.SetField(tb, "done", lua.LBool(t.Done))
	if t.Priority != 0 {
		L.SetField(tb, "priority", lua.LString(string(t.Priority)))
	}
	if d, ok := t.Due(); ok {
		L.SetField(tb, "due", lua.LString(d.Format(todo.DateLayout)))
	}
	list := func(xs []string) *lua.LTable {
		lt := L.CreateTable(len(xs), 0)
		for _, x := range xs {
			lt.Append(lua.LString(x))
		}
		return lt
	}
	L.SetField(tb, "projects", list(t.Projects()))
	L.SetField(tb, "contexts", list(t.Contexts()))
	tags := L.NewTable()
	for _, w := range strings.Fields(t.Text) {
		k, v, ok := strings.Cut(w, ":")
		if ok && k != "" && v != "" && !strings.HasPrefix(v, "//") {
			L.SetField(tags, k, lua.LString(v))
		}
	}
	L.SetField(tb, "tags", tags)
	return tb
}
