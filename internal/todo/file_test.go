package todo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Saving must leave the file as other todo.txt programs wrote it, except
// for the tasks that changed.
func TestSaveKeepsOtherProgramsLines(t *testing.T) {
	for name, c := range map[string]struct{ in, want string }{
		"untouched lines byte for byte": {
			in:   "(A) Call mom  +Family   \nBuy milk @store\t\n",
			want: "(A) Call mom  +Family   \nx 2026-11-19 Buy milk @store\n",
		},
		"Windows line endings": {
			in:   "(A) Call mom\r\nBuy milk\r\n",
			want: "(A) Call mom\r\nx 2026-11-19 Buy milk\r\n",
		},
		"byte order mark": {
			in:   bom + "x 2026-11-18 Old done\nBuy milk\n",
			want: bom + "x 2026-11-18 Old done\nx 2026-11-19 Buy milk\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "todo.txt")
			os.WriteFile(p, []byte(c.in), 0o644)
			ts, _, err := Load(p)
			if err != nil || len(ts) != 2 {
				t.Fatalf("load: %v %d", err, len(ts))
			}
			if name == "byte order mark" && !ts[0].Done {
				t.Fatal("the line after the BOM didn't parse as done")
			}
			ts[1].Complete(time.Date(2026, 11, 19, 9, 0, 0, 0, time.Local))
			if _, err := Save(p, ts); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(p); string(got) != c.want {
				t.Errorf("saved %q\nwant  %q", got, c.want)
			}
		})
	}
}

func TestAppendFollowsDoneFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "done.txt")
	os.WriteFile(p, []byte("x 2026-11-01 Old\r\nx 2026-11-02 Older"), 0o644) // no final newline
	if err := Append(p, []*Task{Parse("x 2026-11-19 New")}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p); string(got) != "x 2026-11-01 Old\r\nx 2026-11-02 Older\r\nx 2026-11-19 New\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestNormalize(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.Local) // a Thursday
	for in, want := range map[string]string{
		"Standup at:9:30":                     "Standup at:930",
		"Dentist at:14:15 due:fri":            "Dentist at:1415 due:2026-11-20",
		"Call at:3:15pm":                      "Call at:315pm",
		"Wake alarm:2026-12-01T07:30":         "Wake alarm:2026-12-01T0730",
		"Already fine at:1430 due:2026-12-01": "Already fine at:1430 due:2026-12-01",
		"Not a time at:noon":                  "Not a time at:noon",
	} {
		tk := Parse(in)
		tk.Normalize(now)
		if tk.String() != want {
			t.Errorf("%q: got %q, want %q", in, tk.String(), want)
		}
	}
}

// A first line holding only a byte order mark is blank, not an empty task.
func TestLoadSkipsBOMOnlyLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "todo.txt")
	os.WriteFile(p, []byte(bom+"\none\n"), 0o644)
	ts, _, err := Load(p)
	if err != nil || len(ts) != 1 || ts[0].Text != "one" {
		t.Fatalf("loaded %d tasks (%v)", len(ts), err)
	}
}
