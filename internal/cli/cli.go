// Package cli implements katTasks' one-shot commands, in the style of
// todo.sh: katTasks add "Call Mom", katTasks ls +Work, katTasks do 3.
package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// Env is what a command needs.
type Env struct {
	Cfg   config.Config
	JSON  bool // machine-readable output
	Force bool // don't ask before deleting
	In    io.Reader
	Out   io.Writer
	Now   func() time.Time
}

type command struct {
	names []string
	usage string
	run   func(c *ctx, args []string) error
}

var commands = []command{
	{[]string{"add", "a"}, "add TEXT...", cmdAdd},
	{[]string{"append", "app"}, "append N TEXT...", cmdAppend},
	{[]string{"prepend", "prep"}, "prepend N TEXT...", cmdPrepend},
	{[]string{"replace"}, "replace N TEXT...", cmdReplace},
	{[]string{"pri", "p"}, "pri N PRIORITY", cmdPri},
	{[]string{"depri", "dp"}, "depri N...", cmdDepri},
	{[]string{"do", "done"}, "do N...", cmdDo},
	{[]string{"del", "rm"}, "del N [TERM]", cmdDel},
	{[]string{"start"}, "start N...", cmdStart},
	{[]string{"stop"}, "stop N...", cmdStop},
	{[]string{"status"}, "status N WORD|none", cmdStatus},
	{[]string{"progress", "prog"}, "progress N PERCENT|none", cmdProgress},
	{[]string{"at", "alert"}, "at N WHEN|none", cmdAt},
	{[]string{"alarm"}, "alarm N WHEN|none", cmdAlarm},
	{[]string{"alerts", "upcoming"}, "alerts [DAYS]", cmdAlerts},
	{[]string{"archive"}, "archive", cmdArchive},
	{[]string{"list", "ls"}, "ls [QUERY...]", cmdList},
	{[]string{"listall", "lsa"}, "lsa [QUERY...]", cmdListAll},
	{[]string{"listpri", "lsp"}, "lsp [PRIORITY or A-C]", cmdListPri},
	{[]string{"listproj", "lsprj"}, "lsprj", cmdListProj},
	{[]string{"listcon", "lsc"}, "lsc", cmdListCon},
}

func find(name string) *command {
	for i := range commands {
		for _, n := range commands[i].names {
			if n == name {
				return &commands[i]
			}
		}
	}
	return nil
}

// IsCommand reports whether name is a subcommand (or alias).
func IsCommand(name string) bool { return find(name) != nil }

// Help is one command's usage line, and the other names it answers to.
type Help struct {
	Usage   string // "add TEXT..."
	Aliases []string
}

// Commands lists the subcommands for katTasks -h.
func Commands() []Help {
	var out []Help
	for _, c := range commands {
		shown, _, _ := strings.Cut(c.usage, " ")
		h := Help{Usage: c.usage}
		for _, n := range c.names {
			if n != shown {
				h.Aliases = append(h.Aliases, n)
			}
		}
		out = append(out, h)
	}
	return out
}

// ErrUsage is returned for bad arguments; the message says what is wrong.
var ErrUsage = errors.New("usage")

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }
func (e usageErr) Unwrap() error { return ErrUsage }

// Run runs a subcommand. args[0] is its name.
func Run(env Env, args []string) error {
	if env.Now == nil {
		env.Now = time.Now
	}
	c := find(args[0])
	if c == nil {
		return usageErr{fmt.Sprintf("unknown command %q", args[0])}
	}
	x := &ctx{Env: env, name: c.names[0], usage: c.usage}
	if err := x.load(); err != nil {
		return err
	}
	return c.run(x, args[1:])
}

// ctx holds the loaded file for one command.
type ctx struct {
	Env
	name, usage string
	tasks       []*todo.Task
}

func (c *ctx) load() error {
	ts, _, err := todo.Load(c.Cfg.Files.Todo)
	c.tasks = ts
	return err
}

func (c *ctx) save() error {
	_, err := todo.Save(c.Cfg.Files.Todo, c.tasks)
	return err
}

func (c *ctx) bad(format string, a ...any) error {
	return usageErr{fmt.Sprintf(format, a...) + "\nusage: katTasks " + c.usage}
}

// num returns task N (1-based).
func (c *ctx) num(s string) (*todo.Task, int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, 0, c.bad("%q is not a task number", s)
	}
	if n < 1 || n > len(c.tasks) {
		return nil, 0, fmt.Errorf("no task %d (there are %d)", n, len(c.tasks))
	}
	return c.tasks[n-1], n, nil
}

func (c *ctx) index(t *todo.Task) int {
	for i, x := range c.tasks {
		if x == t {
			return i + 1
		}
	}
	return 0
}

// ---- output ---------------------------------------------------------------

// Task is the JSON form of a task.
type Task struct {
	N         int               `json:"n"` // 0 for tasks in the done file
	Raw       string            `json:"raw"`
	Done      bool              `json:"done"`
	Priority  string            `json:"priority,omitempty"`
	Created   string            `json:"created,omitempty"`
	Completed string            `json:"completed,omitempty"`
	Text      string            `json:"text"`
	Projects  []string          `json:"projects"`
	Contexts  []string          `json:"contexts"`
	Tags      map[string]string `json:"tags"`
	Due       string            `json:"due,omitempty"`
	At        string            `json:"at,omitempty"`       // time of day, "14:30"
	Alarm     string            `json:"alarm,omitempty"`    // RFC 3339
	Status    string            `json:"status,omitempty"`   // "doing", …
	Progress  *int              `json:"progress,omitempty"` // percent
}

func toJSON(n int, t *todo.Task) Task {
	j := Task{N: n, Raw: t.String(), Done: t.Done, Text: t.Text,
		Projects: nonNil(t.Projects()), Contexts: nonNil(t.Contexts()), Tags: map[string]string{}}
	if t.Priority != 0 {
		j.Priority = string(t.Priority)
	}
	if !t.Created.IsZero() {
		j.Created = t.Created.Format(todo.DateLayout)
	}
	if !t.Completed.IsZero() {
		j.Completed = t.Completed.Format(todo.DateLayout)
	}
	for _, w := range strings.Fields(t.Text) {
		if k, v, ok := strings.Cut(w, ":"); ok && k != "" && v != "" && !strings.HasPrefix(v, "//") && !strings.ContainsAny(k, "+@") {
			if _, dup := j.Tags[k]; !dup {
				j.Tags[k] = v
			}
		}
	}
	if d, ok := t.Due(); ok {
		j.Due = d.Format(todo.DateLayout)
	}
	if v, ok := t.Tag("at"); ok {
		if min, ok := todo.ParseClock(v, true); ok {
			j.At = todo.FormatClock(min)
		}
	}
	if a, ok := t.Alarm(); ok {
		j.Alarm = a.Format(time.RFC3339)
	}
	j.Status, _ = t.Status()
	if p, ok := t.Progress(); ok {
		j.Progress = &p
	}
	return j
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type numbered struct {
	n int
	t *todo.Task
}

// Result is the JSON printed by commands that change tasks.
type Result struct {
	Action  string `json:"action"`
	Tasks   []Task `json:"tasks"`
	Added   []Task `json:"added,omitempty"` // e.g. the next copy of a recurring task
	Message string `json:"message,omitempty"`
}

func (c *ctx) printJSON(v any) error {
	enc := json.NewEncoder(c.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// changed reports tasks a command touched.
func (c *ctx) changed(msg string, ts []numbered, added []numbered) error {
	if c.JSON {
		r := Result{Action: c.name, Tasks: []Task{}, Message: msg}
		for _, x := range ts {
			r.Tasks = append(r.Tasks, toJSON(x.n, x.t))
		}
		for _, x := range added {
			r.Added = append(r.Added, toJSON(x.n, x.t))
		}
		return c.printJSON(r)
	}
	for _, x := range ts {
		fmt.Fprintf(c.Out, "%d %s\n", x.n, x.t)
	}
	for _, x := range added {
		fmt.Fprintf(c.Out, "%d %s\n", x.n, x.t)
	}
	if msg != "" {
		fmt.Fprintf(c.Out, "TODO: %s\n", msg)
	}
	return nil
}

// list prints tasks sorted by their full line, case-insensitively, as
// todo.sh does.
func (c *ctx) list(ts []numbered, total int) error {
	sort.SliceStable(ts, func(i, j int) bool {
		return strings.ToLower(ts[i].t.String()) < strings.ToLower(ts[j].t.String())
	})
	if c.JSON {
		out := []Task{}
		for _, x := range ts {
			out = append(out, toJSON(x.n, x.t))
		}
		return c.printJSON(out)
	}
	w := len(strconv.Itoa(max(total, 1)))
	for _, x := range ts {
		fmt.Fprintf(c.Out, "%0*d %s\n", w, x.n, x.t)
	}
	fmt.Fprintf(c.Out, "--\nTODO: %d of %d tasks shown\n", len(ts), total)
	return nil
}

// ---- commands ----------------------------------------------------------------

func cmdAdd(c *ctx, args []string) error {
	text := strings.TrimSpace(strings.Join(args, " "))
	if text == "" {
		return c.bad("nothing to add")
	}
	t := todo.NewTask(text, c.Now(), c.Cfg.Tasks.AddCreationDate)
	c.tasks = append(c.tasks, t)
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d added.", len(c.tasks)), []numbered{{len(c.tasks), t}}, nil)
}

func textArg(c *ctx, args []string) (*todo.Task, int, string, error) {
	if len(args) < 2 {
		return nil, 0, "", c.bad("need a task number and text")
	}
	t, n, err := c.num(args[0])
	return t, n, strings.TrimSpace(strings.Join(args[1:], " ")), err
}

func cmdAppend(c *ctx, args []string) error {
	t, n, text, err := textArg(c, args)
	if err != nil {
		return err
	}
	t.Text = strings.TrimSpace(t.Text + " " + text)
	t.Normalize(c.Now())
	if err := c.save(); err != nil {
		return err
	}
	return c.changed("", []numbered{{n, t}}, nil)
}

func cmdPrepend(c *ctx, args []string) error {
	t, n, text, err := textArg(c, args)
	if err != nil {
		return err
	}
	t.Text = strings.TrimSpace(text + " " + t.Text)
	t.Normalize(c.Now())
	if err := c.save(); err != nil {
		return err
	}
	return c.changed("", []numbered{{n, t}}, nil)
}

// cmdReplace swaps a task's text. A priority or creation date left out of
// the new text is kept from the old task.
func cmdReplace(c *ctx, args []string) error {
	t, n, text, err := textArg(c, args)
	if err != nil {
		return err
	}
	nt := todo.NewTask(text, c.Now(), false)
	if nt.Priority == 0 && !nt.Done {
		nt.Priority = t.Priority
	}
	if nt.Created.IsZero() {
		nt.Created = t.Created
	}
	old := t.String()
	*t = *nt
	if err := c.save(); err != nil {
		return err
	}
	if !c.JSON {
		fmt.Fprintf(c.Out, "%d %s\n", n, old)
	}
	return c.changed(fmt.Sprintf("Replaced task %d.", n), []numbered{{n, t}}, nil)
}

func cmdPri(c *ctx, args []string) error {
	if len(args) != 2 {
		return c.bad("need a task number and a priority")
	}
	t, n, err := c.num(args[0])
	if err != nil {
		return err
	}
	p := strings.ToUpper(args[1])
	if len(p) != 1 || p[0] < 'A' || p[0] > 'Z' {
		return c.bad("priority must be a letter A-Z")
	}
	if t.Done {
		return fmt.Errorf("task %d is done; completed tasks can't have a priority", n)
	}
	t.Priority = p[0]
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d prioritized (%s).", n, p), []numbered{{n, t}}, nil)
}

// several resolves task numbers to tasks before anything changes, so the
// numbers refer to the file as it was.
func several(c *ctx, args []string) ([]numbered, error) {
	if len(args) == 0 {
		return nil, c.bad("need at least one task number")
	}
	var out []numbered
	seen := map[int]bool{}
	for _, a := range args {
		for _, f := range strings.Split(a, ",") {
			if f == "" {
				continue
			}
			t, n, err := c.num(f)
			if err != nil {
				return nil, err
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, numbered{n, t})
			}
		}
	}
	return out, nil
}

func cmdDepri(c *ctx, args []string) error {
	ts, err := several(c, args)
	if err != nil {
		return err
	}
	for _, x := range ts {
		x.t.Priority = 0
	}
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d deprioritized.", len(ts)), ts, nil)
}

func cmdDo(c *ctx, args []string) error {
	ts, err := several(c, args)
	if err != nil {
		return err
	}
	now := c.Now()
	var nexts []*todo.Task
	var msgs []string
	for _, x := range ts {
		if x.t.Done {
			msgs = append(msgs, fmt.Sprintf("%d is already marked done.", x.n))
			continue
		}
		next, recurs := x.t.Next(now)
		x.t.Complete(now)
		if recurs {
			// After the original, as the TUI does.
			i := c.index(x.t)
			c.tasks = append(c.tasks[:i], append([]*todo.Task{next}, c.tasks[i:]...)...)
			nexts = append(nexts, next)
		}
		msgs = append(msgs, fmt.Sprintf("%d marked as done.", x.n))
	}
	if err := c.save(); err != nil {
		return err
	}
	// Report numbers as they are now that next copies were inserted.
	for i := range ts {
		ts[i].n = c.index(ts[i].t)
	}
	var added []numbered
	for _, n := range nexts {
		added = append(added, numbered{c.index(n), n})
	}
	if len(added) > 0 {
		msgs = append(msgs, fmt.Sprintf("%d recurring task(s) added.", len(added)))
	}
	return c.changed(strings.Join(msgs, " "), ts, added)
}

func cmdDel(c *ctx, args []string) error {
	if len(args) < 1 {
		return c.bad("need a task number")
	}
	t, n, err := c.num(args[0])
	if err != nil {
		return err
	}
	term := strings.TrimSpace(strings.Join(args[1:], " "))
	if term != "" {
		// Remove just TERM from the task.
		before := t.Text
		words := strings.Fields(t.Text)
		var keep []string
		for _, w := range words {
			if w != term {
				keep = append(keep, w)
			}
		}
		t.Text = strings.Join(keep, " ")
		if t.Text == before {
			t.Text = strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(" "+before+" ", " "+term+" ", " ")), " "))
		}
		if t.Text == before {
			return fmt.Errorf("%q not found in task %d", term, n)
		}
		if err := c.save(); err != nil {
			return err
		}
		return c.changed(fmt.Sprintf("Removed '%s' from task.", term), []numbered{{n, t}}, nil)
	}
	if !c.Force {
		if c.JSON {
			return c.bad("deleting with -json needs -f")
		}
		fmt.Fprintf(c.Out, "Delete '%s'?  (y/n) ", t)
		ans, _ := bufio.NewReader(c.In).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
			fmt.Fprintln(c.Out, "TODO: No tasks were deleted.")
			return nil
		}
	}
	c.tasks = append(c.tasks[:n-1], c.tasks[n:]...)
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d deleted.", n), []numbered{{n, t}}, nil)
}

func cmdArchive(c *ctx, args []string) error {
	if len(args) > 0 {
		return c.bad("archive takes no arguments")
	}
	var done, keep []*todo.Task
	var report []numbered
	for i, t := range c.tasks {
		if t.Done {
			done = append(done, t)
			report = append(report, numbered{i + 1, t})
		} else {
			keep = append(keep, t)
		}
	}
	if len(done) > 0 {
		if _, err := todo.Archive(c.Cfg.Files.Todo, keep, c.Cfg.Files.Done, done); err != nil {
			return err
		}
		c.tasks = keep
	}
	return c.changed(fmt.Sprintf("%d task(s) archived to %s.", len(done), c.Cfg.Files.Done), report, nil)
}

func (c *ctx) matching(args []string) []numbered {
	q := query.Parse(strings.Join(args, " "))
	now := c.Now()
	var out []numbered
	for i, t := range c.tasks {
		if q.Match(t, now) {
			out = append(out, numbered{i + 1, t})
		}
	}
	return out
}

func cmdList(c *ctx, args []string) error {
	return c.list(c.matching(args), len(c.tasks))
}

func cmdListAll(c *ctx, args []string) error {
	ts := c.matching(args)
	done, _, err := todo.Load(c.Cfg.Files.Done)
	if err != nil {
		return err
	}
	q := query.Parse(strings.Join(args, " "))
	now := c.Now()
	for _, t := range done {
		if q.Match(t, now) {
			ts = append(ts, numbered{0, t})
		}
	}
	return c.list(ts, len(c.tasks)+len(done))
}

func cmdListPri(c *ctx, args []string) error {
	if len(args) > 1 {
		return c.bad("one priority or range at most")
	}
	q := "-pri:none"
	if len(args) == 1 {
		p := strings.ToUpper(args[0])
		letter := func(b byte) bool { return b >= 'A' && b <= 'Z' }
		if !(len(p) == 1 && letter(p[0])) && !(len(p) == 3 && letter(p[0]) && p[1] == '-' && letter(p[2])) {
			return c.bad("%q is not a priority (want a letter like A, or a range like A-C)", args[0])
		}
		q = "pri:" + p
	}
	ts := c.matching([]string{q})
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].t.Priority < ts[j].t.Priority })
	if c.JSON {
		out := []Task{}
		for _, x := range ts {
			out = append(out, toJSON(x.n, x.t))
		}
		return c.printJSON(out)
	}
	w := len(strconv.Itoa(max(len(c.tasks), 1)))
	for _, x := range ts {
		fmt.Fprintf(c.Out, "%0*d %s\n", w, x.n, x.t)
	}
	fmt.Fprintf(c.Out, "--\nTODO: %d of %d tasks shown\n", len(ts), len(c.tasks))
	return nil
}

func (c *ctx) words(get func(*todo.Task) []string) error {
	seen := map[string]bool{}
	var out []string
	for _, t := range c.tasks {
		for _, w := range get(t) {
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	if c.JSON {
		return c.printJSON(nonNil(out))
	}
	for _, w := range out {
		fmt.Fprintln(c.Out, w)
	}
	return nil
}

func cmdListProj(c *ctx, _ []string) error { return c.words((*todo.Task).Projects) }
func cmdListCon(c *ctx, _ []string) error  { return c.words((*todo.Task).Contexts) }
