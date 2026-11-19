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

func TestWorkCommands(t *testing.T) {
	f := newFixture(t, "Write report +Work", "Call mom", "x 2026-11-17 Old")
	f.ok("start", "1")
	f.ok("progress", "1", "40")
	f.ok("progress", "1", "+15%")
	f.ok("status", "2", "Blocked")
	if got := f.file(); !strings.Contains(got, "Write report +Work status:doing progress:55\nCall mom status:blocked\n") {
		t.Fatalf("file:\n%s", got)
	}
	f.ok("stop", "1", "2")
	f.ok("progress", "1", "none")
	if got := f.file(); !strings.HasPrefix(got, "Write report +Work\nCall mom\n") {
		t.Fatalf("after stop:\n%s", got)
	}
	if _, err := f.run("", false, false, "start", "3"); err == nil {
		t.Error("started a done task")
	}
	for _, bad := range [][]string{{"progress", "1", "150"}, {"status", "1", "a:b"}, {"at", "1", "25:00"}, {"alarm", "1", "someday"}} {
		if _, err := f.run("", false, false, bad...); !errors.Is(err, ErrUsage) {
			t.Errorf("%v: %v", bad, err)
		}
	}
}

func TestAtAlarmAndAlerts(t *testing.T) {
	f := newFixture(t, "Standup +Work", "Dentist", "Water plants")
	f.ok("at", "1", "9:30") // written without the colon
	f.ok("alarm", "2", "tomorrow", "7am")
	f.ok("alarm", "3", "8:00") // before now (9:00), so tomorrow
	got := f.file()
	for _, want := range []string{"Standup +Work at:930\n", "Dentist alarm:2026-11-19T0700\n", "Water plants alarm:2026-11-19T0800\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lacks %q:\n%s", want, got)
		}
	}
	f.ok("alarm", "3", "fri", "1830")
	if !strings.Contains(f.file(), "Water plants alarm:2026-11-20T1830") {
		t.Errorf("fri alarm:\n%s", f.file())
	}

	// The next 2 days from Wed 9:00: standup's alert at 9:20 (lead 10m) each
	// day and the alarms, soonest first; Fri 18:30 is past the window.
	out := f.ok("alerts", "2")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"Wed Nov 18  9:20  alert  1 ", "Thu Nov 19  7:00  alarm  2 ", "Thu Nov 19  9:20  alert  1 "}
	if len(lines) != len(want)+2 {
		t.Fatalf("alerts:\n%s", out)
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Errorf("line %d = %q, want %q…", i, lines[i], w)
		}
	}

	js, err := f.run("", true, false, "ls")
	if err != nil {
		t.Fatal(err)
	}
	var ts []Task
	if err := json.Unmarshal([]byte(js), &ts); err != nil {
		t.Fatal(err)
	}
	for _, x := range ts {
		if x.N == 1 && x.At != "9:30" {
			t.Errorf("json at %q", x.At)
		}
		if x.N == 2 && !strings.HasPrefix(x.Alarm, "2026-11-19T07:00:00") {
			t.Errorf("json alarm %q", x.Alarm)
		}
	}

	f.ok("alarm", "2", "none")
	f.ok("at", "1", "none")
	if strings.Contains(f.file(), "Dentist alarm:") || strings.Contains(f.file(), "at:") {
		t.Errorf("not cleared:\n%s", f.file())
	}
}

func TestRelativeAlertsAndAlarms(t *testing.T) {
	f := newFixture(t, "Standup") // now is Wed Nov 18 9:00; alerts.lead is 10m
	f.ok("alarm", "1", "+5m")
	if !strings.Contains(f.file(), "Standup alarm:2026-11-18T0905") {
		t.Fatalf("alarm +5m:\n%s", f.file())
	}
	// An alert in 5 minutes: due today, at: lead after that, so it goes off at 9:05.
	f.ok("alert", "1", "in", "5m")
	if !strings.Contains(f.file(), "due:2026-11-18 at:915") {
		t.Fatalf("alert in 5m:\n%s", f.file())
	}
	if out := f.ok("alerts", "1"); !strings.Contains(out, "Wed Nov 18  9:05  alert  1 ") || !strings.Contains(out, "Wed Nov 18  9:05  alarm  1 ") {
		t.Errorf("alerts:\n%s", out)
	}

	// With text instead of a number, they add a task.
	out := f.ok("alarm", "+1h30m", "Tea", "is", "ready")
	if !strings.Contains(f.file(), "2026-11-18 Tea is ready alarm:2026-11-18T1030\n") || !strings.Contains(out, "2 added.") {
		t.Fatalf("new alarm (%s):\n%s", out, f.file())
	}
	f.ok("alert", "+5m", "Check", "the", "oven")
	if !strings.Contains(f.file(), "Check the oven due:2026-11-18 at:915\n") {
		t.Fatalf("new alert:\n%s", f.file())
	}
	f.ok("alarm", "tomorrow", "6:30", "Wake", "up")
	f.ok("alarm", "fri", "Gym") // a date alone rings at alarms.default_time
	for _, want := range []string{"Wake up alarm:2026-11-19T0630", "Gym alarm:2026-11-20T0700"} {
		if !strings.Contains(f.file(), want) {
			t.Errorf("file lacks %q:\n%s", want, f.file())
		}
	}

	// Late in the day the alert's at: time and due: move to tomorrow.
	late := newFixture(t, "Lock up")
	late.cfg.Alerts.LeadTime = 10 * time.Minute
	if err := Run(Env{Cfg: late.cfg, Out: &bytes.Buffer{}, Now: func() time.Time { return time.Date(2026, 11, 18, 23, 55, 0, 0, time.Local) }}, []string{"alert", "1", "+1m"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(late.file(), "Lock up due:2026-11-19 at:0006") {
		t.Errorf("across midnight:\n%s", late.file())
	}

	for _, bad := range [][]string{{"alarm", "+5m"}, {"alert", "soon", "x"}, {"alarm", "+0m", "x"}} {
		if _, err := f.run("", false, false, bad...); !errors.Is(err, ErrUsage) {
			t.Errorf("%v: %v", bad, err)
		}
	}
}

func TestShortDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{20 * time.Minute: "20m", 2 * time.Hour: "2h", 90 * time.Minute: "1h30m", 90 * time.Second: "1m30s"} {
		if got := shortDuration(d); got != want {
			t.Errorf("%v: %q", d, got)
		}
	}
}
