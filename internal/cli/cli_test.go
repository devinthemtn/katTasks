package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
)

// Wednesday 2026-11-18.
var now = time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local)

type fixture struct {
	t          *testing.T
	todo, done string
	cfg        config.Config
}

func newFixture(t *testing.T, lines ...string) *fixture {
	dir := t.TempDir()
	f := &fixture{t: t, todo: filepath.Join(dir, "todo.txt"), done: filepath.Join(dir, "done.txt")}
	if err := os.WriteFile(f.todo, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.cfg = config.Defaults()
	f.cfg.SetTodo(f.todo)
	return f
}

func (f *fixture) run(stdin string, jsonOut, force bool, args ...string) (string, error) {
	var out bytes.Buffer
	err := Run(Env{Cfg: f.cfg, JSON: jsonOut, Force: force, In: strings.NewReader(stdin), Out: &out, Now: func() time.Time { return now }}, args)
	return out.String(), err
}

func (f *fixture) ok(args ...string) string {
	f.t.Helper()
	out, err := f.run("", false, false, args...)
	if err != nil {
		f.t.Fatalf("%v: %v", args, err)
	}
	return out
}

func (f *fixture) file() string {
	b, _ := os.ReadFile(f.todo)
	return string(b)
}

func TestAddAndList(t *testing.T) {
	f := newFixture(t, "(B) Write report +Work due:2026-11-20", "buy milk +Home")
	out := f.ok("add", "Call", "Mom", "+Family", "due:fri")
	if !strings.Contains(out, "3 2026-11-18 Call Mom +Family due:2026-11-20") || !strings.Contains(out, "TODO: 3 added.") {
		t.Errorf("add output:\n%s", out)
	}
	out = f.ok("ls")
	want := "1 (B) Write report +Work due:2026-11-20\n3 2026-11-18 Call Mom +Family due:2026-11-20\n2 buy milk +Home\n--\nTODO: 3 of 3 tasks shown\n"
	if out != want {
		t.Errorf("ls:\n%s\nwant:\n%s", out, want)
	}
	out = f.ok("ls", "due:+1w", "-+Work")
	if !strings.HasPrefix(out, "3 2026-11-18 Call Mom") || !strings.Contains(out, "1 of 3") {
		t.Errorf("ls query:\n%s", out)
	}
	if got := f.ok("lsprj"); got != "+Family\n+Home\n+Work\n" {
		t.Errorf("lsprj = %q", got)
	}
}

func TestEdits(t *testing.T) {
	f := newFixture(t, "(A) 2026-11-01 one", "two @home")
	f.ok("append", "1", "+Proj")
	f.ok("prepend", "2", "really")
	f.ok("pri", "2", "c")
	f.ok("replace", "1", "uno due:tomorrow")
	f.ok("depri", "2")
	if got := f.file(); got != "(A) 2026-11-01 uno due:2026-11-19\nreally two @home\n" {
		t.Errorf("file:\n%s", got)
	}
	f.ok("del", "2", "@home")
	if !strings.Contains(f.file(), "really two\n") {
		t.Errorf("del TERM:\n%s", f.file())
	}
}

func TestDoRecurringAndArchive(t *testing.T) {
	f := newFixture(t, "Pay rent due:2026-11-01 rec:+1m", "Water plants rec:1w", "Other")
	out := f.ok("do", "1", "2")
	file := f.file()
	want := "x 2026-11-18 Pay rent due:2026-11-01 rec:+1m\nPay rent due:2026-12-01 rec:+1m\n" +
		"x 2026-11-18 Water plants rec:1w\nWater plants rec:1w due:2026-11-25\nOther\n"
	if file != want {
		t.Errorf("file after do:\n%s\nwant:\n%s", file, want)
	}
	if !strings.Contains(out, "2 recurring task(s) added.") || !strings.Contains(out, "\n3 x 2026-11-18 Water plants") {
		t.Errorf("do output:\n%s", out)
	}
	f.ok("archive")
	if f.file() != "Pay rent due:2026-12-01 rec:+1m\nWater plants rec:1w due:2026-11-25\nOther\n" {
		t.Errorf("after archive:\n%s", f.file())
	}
	d, _ := os.ReadFile(f.done)
	if strings.Count(string(d), "\n") != 2 {
		t.Errorf("done.txt:\n%s", d)
	}
	if out := f.ok("lsa"); !strings.Contains(out, "0 x 2026-11-18 Pay rent") || !strings.Contains(out, "5 of 5") {
		t.Errorf("lsa:\n%s", out)
	}
}

func TestDeleteConfirm(t *testing.T) {
	f := newFixture(t, "keep", "drop")
	out, _ := f.run("n\n", false, false, "del", "2")
	if !strings.Contains(out, "No tasks were deleted") || !strings.Contains(f.file(), "drop") {
		t.Errorf("declined delete:\n%s", out)
	}
	f.run("y\n", false, false, "rm", "2")
	if f.file() != "keep\n" {
		t.Errorf("after confirmed delete: %q", f.file())
	}
	f.run("", false, true, "del", "1")
	if f.file() != "" {
		t.Errorf("after forced delete: %q", f.file())
	}
}

func TestErrors(t *testing.T) {
	f := newFixture(t, "only")
	for _, args := range [][]string{{"do", "2"}, {"do", "x"}, {"pri", "1", "AA"}, {"add"}, {"lsp", "nope"}} {
		_, err := f.run("", false, false, args...)
		if err == nil {
			t.Errorf("%v: no error", args)
		}
	}
	_, err := f.run("", false, false, "pri", "1")
	if !errors.Is(err, ErrUsage) || !strings.Contains(err.Error(), "usage: katTasks pri N PRIORITY") {
		t.Errorf("usage error = %v", err)
	}
}

func TestJSON(t *testing.T) {
	f := newFixture(t, "(A) 2026-11-01 Call Mom +Family @phone due:2026-11-20 uid:7", "x 2026-11-02 done thing")
	out, err := f.run("", true, false, "ls", "+Family")
	if err != nil {
		t.Fatal(err)
	}
	var tasks []Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want := Task{N: 1, Raw: "(A) 2026-11-01 Call Mom +Family @phone due:2026-11-20 uid:7", Priority: "A", Created: "2026-11-01",
		Text: "Call Mom +Family @phone due:2026-11-20 uid:7", Projects: []string{"+Family"}, Contexts: []string{"@phone"},
		Tags: map[string]string{"due": "2026-11-20", "uid": "7"}, Due: "2026-11-20"}
	got, _ := json.Marshal(tasks)
	exp, _ := json.Marshal([]Task{want})
	if string(got) != string(exp) {
		t.Errorf("json:\n%s\nwant:\n%s", got, exp)
	}

	out, _ = f.run("", true, false, "do", "1")
	var r Result
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.Action != "do" || len(r.Tasks) != 1 || !r.Tasks[0].Done {
		t.Errorf("do json: %v\n%s", err, out)
	}
	if _, err := f.run("", true, false, "del", "1"); !errors.Is(err, ErrUsage) {
		t.Errorf("del with -json and no -f should refuse, got %v", err)
	}
}
