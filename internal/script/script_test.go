package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/todo"
)

func write(t *testing.T, dir, name, src string) Script {
	t.Helper()
	p := filepath.Join(dir, name+".lua")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return Script{Name: name, Path: p}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "b", "return 1")
	write(t, dir, "a", "-- says hello\nreturn 'hi'")
	os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644)
	got := List(dir)
	if len(got) != 2 || got[0].Name != "a" || got[0].Desc != "says hello" || got[1].Desc != "" {
		t.Fatalf("List = %+v", got)
	}
	if List(filepath.Join(dir, "missing")) != nil {
		t.Error("missing folder should have no scripts")
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local) // a Wednesday
	cur := todo.Parse("(B) Call mom +Family @phone due:2026-11-20 id:7")
	env := Env{Inline: true, Input: "Pay rent", Current: cur, Tasks: []*todo.Task{cur}, Now: now}

	s := write(t, dir, "x", `
kat.add("New one +Home")
print("hello", 2)
local t = kat.task
return kat.input .. "|" .. kat.date("fri") .. "|" .. t.priority .. t.due .. t.projects[1] .. t.tags.id .. "|" .. #kat.tasks() .. kat.today`)
	res, err := Run(s, env)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Pay rent|2026-11-20|B2026-11-20+Family7|12026-11-18"; res.Output != want {
		t.Errorf("output %q, want %q", res.Output, want)
	}
	if len(res.Adds) != 1 || res.Adds[0] != "New one +Home" || res.Status != "hello 2" {
		t.Errorf("result %+v", res)
	}

	if _, err := Run(write(t, dir, "u", `kat.update("x")`), env); err == nil || !strings.Contains(err.Error(), "only works") {
		t.Errorf("inline update: %v", err)
	}
	env.Inline = false
	res, err = Run(write(t, dir, "u", `kat.update(kat.task.text .. " done")`), env)
	if err != nil || res.Update == nil || !strings.HasSuffix(*res.Update, "id:7 done") {
		t.Errorf("update: %+v %v", res, err)
	}
	if _, err := Run(write(t, dir, "bad", `error("boom")`), env); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error: %v", err)
	}
	if _, err := Run(write(t, dir, "loop", `while true do end`), env); err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Errorf("timeout: %v", err)
	}
	if res, _ := Run(write(t, dir, "nil", `return nil`), env); res.HasOutput {
		t.Error("nil should give no output")
	}
}
