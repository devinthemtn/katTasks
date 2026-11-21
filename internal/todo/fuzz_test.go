package todo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fuzz tests for the parsers whose output goes back into todo.txt. Plain
// go test runs the seeds below; go test -fuzz=FuzzX explores further.

var lineSeeds = []string{
	"(A) Call Mom +Family @phone",
	"(B) 2026-01-02 Write report +Work due:2026-02-01 rec:+1m t:2026-01-25",
	"x 2026-01-05 2026-01-02 Pay rent pri:A",
	"x 2026-01-05 Done",
	"2026-03-04",
	"x ",
	"(Z) ",
	"  leading and trailing spaces  ",
	"tabs\tin\ttext\t",
	"Wake up alarm:2026-12-01T0730 at:7am status:doing progress:40",
	"(a) lowercase x (B) not a priority",
	"2026-02-30 not a date",
	bom + "(A) after a byte order mark",
	"ünïcödé ✓ 日本語 +プロジェクト",
}

// An untouched line is written back exactly as read, and a task's String
// reads back as the same task.
func FuzzParseLine(f *testing.F) {
	for _, s := range lineSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		if strings.ContainsAny(line, "\r\n") || strings.Contains(strings.TrimPrefix(line, bom), bom) {
			return // one line only; a mark inside the text reads as the file's
		}
		task := Parse(line)
		if got, want := task.line(), strings.TrimPrefix(line, bom); got != want {
			t.Fatalf("untouched line written back as %q, want %q", got, want)
		}
		s := task.String()
		again := Parse(s)
		if again.String() != s || again.Done != task.Done || again.Priority != task.Priority ||
			!again.Created.Equal(task.Created) || !again.Completed.Equal(task.Completed) {
			t.Fatalf("String %q of %q reads back as %+v", s, line, again)
		}

		// Changing it doesn't panic, and still writes one line that reads
		// back as itself.
		now := time.Date(2026, 3, 4, 9, 0, 0, 0, time.Local)
		if n, ok := task.Next(now); ok {
			checkLine(t, n.String())
		}
		task.Complete(now)
		checkLine(t, task.String())
		task.Uncomplete()
		checkLine(t, task.String())
		task.Normalize(now)
		checkLine(t, task.String())
	})
}

// checkLine checks a changed task's line: one line, which settles after
// one read. It may not read back exactly: text that starts with a date
// becomes the creation date once a priority goes in front of it, as with
// todo.sh, since the format can't tell them apart.
func checkLine(t *testing.T, s string) {
	t.Helper()
	if strings.ContainsAny(s, "\r\n") {
		t.Fatalf("line break in %q", s)
	}
	once := Parse(s).String()
	if twice := Parse(once).String(); twice != once {
		t.Fatalf("%q reads back as %q, then as %q", s, once, twice)
	}
}

// Whatever the lines, saving and loading keeps every task's line.
func FuzzSaveLoad(f *testing.F) {
	f.Add(strings.Join(lineSeeds, "\n"), false)
	f.Add("one\r\ntwo\r\n", true)
	f.Fuzz(func(t *testing.T, content string, crlf bool) {
		var tasks []*Task
		for _, l := range strings.Split(content, "\n") {
			l = strings.TrimRight(l, "\r")
			// Byte order marks have their own test (TestSaveKeepsOtherProgramsLines).
			if strings.TrimSpace(l) == "" || strings.ContainsAny(l, "\r"+bom) || len(l) > 64*1024 {
				continue
			}
			tasks = append(tasks, Parse(l))
		}
		p := filepath.Join(t.TempDir(), "todo.txt")
		if crlf {
			os.WriteFile(p, []byte("seed\r\n"), 0o644)
		}
		if _, err := Save(p, tasks); err != nil {
			t.Fatal(err)
		}
		got, _, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tasks) {
			t.Fatalf("saved %d tasks, loaded %d", len(tasks), len(got))
		}
		for i := range tasks {
			if got[i].line() != tasks[i].line() {
				t.Fatalf("task %d: saved %q, loaded %q", i, tasks[i].line(), got[i].line())
			}
		}
	})
}

// The small parsers: no panics, no hangs, and what parses formats back to
// the same value.
func FuzzParseValues(f *testing.F) {
	for _, s := range []string{"14:30", "9.30", "3pm", "315pm", "9", "12am", "2026-12-01T0730", "1w", "+2m", "999999999b", "0d", "40%", "100", "today", "fri", "+3d", "+10w", "+999999999999m"} {
		f.Add(s)
	}
	now := time.Date(2026, 3, 4, 9, 0, 0, 0, time.Local)
	f.Fuzz(func(t *testing.T, v string) {
		if min, ok := ParseClock(v, true); ok {
			if min < 0 || min >= 24*60 {
				t.Fatalf("ParseClock(%q) = %d", v, min)
			}
			if back, ok := ParseClock(FormatClock(min), false); !ok || back != min {
				t.Fatalf("clock %q = %d formats as %q, which reads as %d %v", v, min, FormatClock(min), back, ok)
			}
		}
		if a, ok := ParseAlarm(v); ok && a.Year() >= 1 && a.Year() <= 9999 {
			if back, ok := ParseAlarm(FormatAlarm(a)); !ok || !back.Equal(a) {
				t.Fatalf("alarm %q = %v formats as %q, which reads as %v %v", v, a, FormatAlarm(a), back, ok)
			}
		}
		if r, ok := ParseRecurrence(v); ok {
			if r.N < 1 {
				t.Fatalf("ParseRecurrence(%q) = %+v", v, r)
			}
			if r.N <= 10000 && !r.Add(now).After(now) {
				t.Fatalf("rec:%s doesn't move the date forward", v)
			}
		}
		ParsePercent(v)
		ResolveDate(v, now)
	})
}
