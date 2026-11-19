package alert

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/todo"
)

func at(h, m int) time.Time { return time.Date(2026, 11, 19, h, m, 0, 0, time.Local) }

func parse(lines ...string) []*todo.Task {
	var ts []*todo.Task
	for _, l := range lines {
		ts = append(ts, todo.Parse(l))
	}
	return ts
}

func TestPending(t *testing.T) {
	tasks := parse(
		"Standup at:9:30 +Work",           // every day
		"Dentist at:14:00 due:2026-11-19", // today
		"Dentist again at:14:00 due:2026-11-20",
		"x 2026-11-19 Done already at:9:30",
		"Call mom 9:30", // a time in the text isn't an alert
		"Bad time at:25:00",
	)
	lead := 10 * time.Minute
	cases := []struct {
		now  time.Time
		want []string
	}{
		{at(9, 19), nil},
		{at(9, 20), []string{"Standup"}},
		{at(9, 34), []string{"Standup"}},
		{at(9, 35), nil}, // CatchUp passed
		{at(13, 50), []string{"Dentist"}},
	}
	for _, c := range cases {
		var got []string
		for _, a := range Pending(tasks, c.now, lead) {
			got = append(got, a.Text)
		}
		if len(got) != len(c.want) || (len(got) > 0 && got[0] != c.want[0]) {
			t.Errorf("%s: got %q, want %q", c.now.Format("15:04"), got, c.want)
		}
	}
	a := Pending(tasks, at(9, 20), lead)[0]
	if !a.At.Equal(at(9, 30)) || !a.FireAt.Equal(at(9, 20)) {
		t.Errorf("times %v %v", a.At, a.FireAt)
	}
	if title, body := a.Message(); title != "katTasks" || body != "9:30  Standup" {
		t.Errorf("message %q %q", title, body)
	}
}

func TestClaimOnce(t *testing.T) {
	dir := t.TempDir()
	a := Alert{Text: "Standup", At: at(9, 30)}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := Claim(dir, a); err != nil {
				t.Error(err)
			} else if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("%d claims won", wins)
	}
	if ok, _ := Claim(dir, Alert{Text: "Standup", At: at(9, 30).AddDate(0, 0, 1)}); !ok {
		t.Error("the next day's alert was already claimed")
	}
}

func TestCheckSendsOnceAndPrunes(t *testing.T) {
	dir := t.TempDir()
	todoPath := filepath.Join(dir, "todo.txt")
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(todoPath, []byte("Standup at:9:30\n"), 0o644)
	os.WriteFile(cfgPath, []byte("[files]\ntodo = "+config.String(todoPath)+"\n[alerts]\nlead = \"0m\"\n"), 0o644)

	old := filepath.Join(dir, "markers", "old")
	os.MkdirAll(filepath.Dir(old), 0o755)
	os.WriteFile(old, nil, 0o644)
	os.Chtimes(old, at(0, 0).AddDate(0, 0, -5), at(0, 0).AddDate(0, 0, -5))

	var sent []string
	now := at(9, 29)
	o := Options{
		Source: FileSource(cfgPath, ""),
		Send:   func(a Alert, s Settings) error { sent = append(sent, a.Text); return nil },
		Dir:    filepath.Join(dir, "markers"),
		Now:    func() time.Time { return now },
	}
	for _, n := range []time.Time{at(9, 29), at(9, 30), at(9, 31)} {
		now = n
		if err := o.Check(); err != nil {
			t.Fatal(err)
		}
	}
	// A second process with its own source still doesn't repeat it.
	o2 := o
	o2.Source = FileSource(cfgPath, "")
	o2.Check()
	if len(sent) != 1 || sent[0] != "Standup" {
		t.Fatalf("sent %q", sent)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("old marker not pruned")
	}

	// Disabled: nothing is sent.
	os.WriteFile(cfgPath, []byte("[files]\ntodo = "+config.String(todoPath)+"\n[alerts]\nenabled = false\n"), 0o644)
	os.Chtimes(cfgPath, time.Now().Add(time.Minute), time.Now().Add(time.Minute))
	os.WriteFile(todoPath, []byte("Lunch at:12:00\n"), 0o644)
	os.Chtimes(todoPath, time.Now().Add(time.Minute), time.Now().Add(time.Minute))
	now = at(11, 55)
	o.Check()
	if len(sent) != 1 {
		t.Fatalf("sent while disabled: %q", sent)
	}
}

func TestAlarms(t *testing.T) {
	tasks := parse(
		"Wake up alarm:2026-11-19T07:30",
		"x 2026-11-18 Done alarm:2026-11-19T07:30",
		"Bad alarm:tomorrow",
		"Tomorrow alarm:2026-11-20T07:30",
	)
	for now, want := range map[time.Time]int{at(7, 29): 0, at(7, 30): 1, at(7, 44): 1, at(7, 45): 0} {
		got := Alarms(tasks, now)
		if len(got) != want {
			t.Errorf("%s: %d alarms, want %d", now.Format("15:04"), len(got), want)
		}
		if len(got) == 1 && (!got[0].Alarm || got[0].Text != "Wake up") {
			t.Errorf("alarm %+v", got[0])
		}
	}
	// An alarm and an at: alert at the same time are separate markers.
	dir := t.TempDir()
	a := Alert{Text: "Wake up", At: at(7, 30)}
	Claim(dir, a)
	a.Alarm = true
	if ok, _ := Claim(dir, a); !ok {
		t.Error("the alarm was taken for the at: alert")
	}
}

func TestCheckRingsAlarms(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache")) // for the built-in sound
	todoPath := filepath.Join(dir, "todo.txt")
	cfgPath := filepath.Join(dir, "config.toml")
	os.WriteFile(todoPath, []byte("Wake up alarm:2026-11-19T07:30\n"), 0o644)
	os.WriteFile(cfgPath, []byte("[files]\ntodo = "+config.String(todoPath)+"\n[alarms]\nring = \"5s\"\n"), 0o644)
	var got []Settings
	o := Options{
		Source: FileSource(cfgPath, ""),
		Send:   func(a Alert, s Settings) error { got = append(got, s); return nil },
		Dir:    filepath.Join(dir, "markers"),
		Now:    func() time.Time { return at(7, 31) },
	}
	if err := o.Check(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Ring != 5*time.Second || !got[0].Urgent || got[0].SoundFile == "" {
		t.Fatalf("settings %+v", got)
	}
	if b, err := os.ReadFile(got[0].SoundFile); err != nil || string(b[:4]) != "RIFF" {
		t.Errorf("built-in alarm sound: %v", err)
	}
}
