package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The -json output is part of what v1 promises: scripts parse it. These
// tests pin it byte for byte, so a change to its shape has to be made on
// purpose. After a deliberate change, regenerate the files with
//
//	go test ./internal/cli -run TestJSONGolden -update
//
// and say what changed in CHANGELOG.md.
var update = flag.Bool("update", false, "rewrite the golden files in testdata/json")

func TestJSONGolden(t *testing.T) {
	// Alarm and alert times print with the local offset. Pin it, so the
	// files read the same in every time zone.
	saved := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = saved })
	at := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)

	lines := []string{
		"(A) 2026-11-01 Call Mom +Family @phone due:2026-11-20 at:1430 uid:7",
		"(B) Write report +Work status:doing progress:40 alarm:2026-11-19T0530",
		"2026-11-10 Water plants @home rec:+1w due:2026-11-18",
		"x 2026-11-17 2026-11-02 Pay rent +Home pri:A",
		"plain task",
	}
	cases := []struct {
		name string
		args []string
	}{
		{"ls", []string{"ls"}},
		{"ls-empty", []string{"ls", "+Nothing"}},
		{"lsa", []string{"lsa"}},
		{"lsp", []string{"lsp", "A-B"}},
		{"lsprj", []string{"lsprj"}},
		{"lsc", []string{"lsc"}},
		{"alerts", []string{"alerts", "3"}},
		{"add", []string{"add", "(C)", "New", "thing", "+Work", "due:fri"}},
		{"do-recurring", []string{"do", "3"}},
		{"del", []string{"del", "5"}},
		{"del-term", []string{"del", "1", "uid:7"}},
		{"pri", []string{"pri", "5", "a"}},
		{"start", []string{"start", "1"}},
		{"progress", []string{"progress", "2", "75"}},
		{"alert", []string{"alert", "5", "930"}},
		{"archive", []string{"archive"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, lines...)
			out, err := f.runAt(at, tc.args...)
			if err != nil {
				t.Fatalf("%v: %v", tc.args, err)
			}
			path := filepath.Join("testdata", "json", tc.name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update to create it)", err)
			}
			if out != string(want) {
				t.Errorf("%v -json changed.\ngot:\n%s\nwant (%s):\n%s", tc.args, out, path, want)
			}
		})
	}
}

// runAt runs a command with -json and -f at a fixed time.
func (f *fixture) runAt(at time.Time, args ...string) (string, error) {
	f.t.Helper()
	var out bytes.Buffer
	err := Run(Env{Cfg: f.cfg, JSON: true, Force: true, In: strings.NewReader(""), Out: &out, Now: func() time.Time { return at }}, args)
	// done.txt is in a temporary directory; messages that name it would
	// differ on every run and on every OS.
	done, _ := json.Marshal(f.done)
	return strings.ReplaceAll(out.String(), strings.Trim(string(done), `"`), "$DIR/done.txt"), err
}
