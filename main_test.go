package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// katTasks runs katTasks with args, against a settings file and todo.txt
// in a fresh folder unless args name others, and returns the exit code and
// what it printed.
func katTasks(t *testing.T, dir, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	full := append([]string{"-config", filepath.Join(dir, "config.toml"), "-file", filepath.Join(dir, "todo.txt")}, args...)
	code = run(full, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionAndHelp(t *testing.T) {
	dir := t.TempDir()
	if code, out, _ := katTasks(t, dir, "", "-version"); code != 0 || !strings.HasPrefix(out, "katTasks ") {
		t.Errorf("-version: %d %q", code, out)
	}
	for _, args := range [][]string{{"-h"}, {"help"}} {
		code, out, errOut := katTasks(t, dir, "", args...)
		if code != 0 || !strings.Contains(out, "Usage") || errOut != "" {
			t.Errorf("%v: code %d, stdout %d bytes, stderr %q; want the help on stdout", args, code, len(out), errOut)
		}
	}
}

func TestMistakesExit2(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := katTasks(t, dir, "", "-nosuchflag")
	if code != 2 || out != "" || !strings.Contains(errOut, "nosuchflag") || !strings.Contains(errOut, "Usage") {
		t.Errorf("bad flag: code %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = katTasks(t, dir, "", "frobnicate")
	if code != 2 || !strings.Contains(errOut, `unknown command "frobnicate"`) {
		t.Errorf("unknown command: code %d, stderr %q", code, errOut)
	}
	code, _, errOut = katTasks(t, dir, "", "do")
	if code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("command missing its argument: code %d, stderr %q", code, errOut)
	}
}

func TestPaths(t *testing.T) {
	dir := t.TempDir()
	code, out, _ := katTasks(t, dir, "", "-paths")
	want := "settings: " + filepath.Join(dir, "config.toml") + "\ntodo: " + filepath.Join(dir, "todo.txt") + "\ndone: " + filepath.Join(dir, "done.txt") + "\n"
	if code != 0 || out != want {
		t.Errorf("-paths: %d\n%s\nwant\n%s", code, out, want)
	}
}

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	if code, out, errOut := katTasks(t, dir, "", "add", "Call mom +Family"); code != 0 || !strings.Contains(out, "added") {
		t.Fatalf("add: %d %q %q", code, out, errOut)
	}
	if code, out, _ := katTasks(t, dir, "", "-json", "ls"); code != 0 || !strings.HasPrefix(strings.TrimSpace(out), "[") || !strings.Contains(out, "Call mom +Family") {
		t.Errorf("-json ls: %d %q", code, out)
	}
	if code, _, _ := katTasks(t, dir, "", "do", "1"); code != 0 {
		t.Errorf("do 1: %d", code)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "todo.txt"))
	if !strings.HasPrefix(string(b), "x ") {
		t.Errorf("todo.txt after do:\n%s", b)
	}
	// An error in a command exits 1.
	if code, _, errOut := katTasks(t, dir, "", "do", "99"); code == 0 || errOut == "" {
		t.Errorf("do 99: %d %q", code, errOut)
	}
}

func TestMakeAndUpdateConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	if code, out, _ := katTasks(t, dir, "", "-makeconfig"); code != 0 || out != "created "+cfg+"\n" {
		t.Errorf("-makeconfig: %d %q", code, out)
	}
	if code, out, _ := katTasks(t, dir, "", "-makeconfig"); code != 0 || !strings.Contains(out, "already exists") {
		t.Errorf("second -makeconfig: %d %q", code, out)
	}
	if code, out, _ := katTasks(t, dir, "", "-updateconfig"); code != 0 || !strings.Contains(out, "already has every setting") {
		t.Errorf("-updateconfig on a full file: %d %q", code, out)
	}
	os.WriteFile(cfg, []byte("theme = \"default\"\n"), 0o644)
	if code, out, _ := katTasks(t, dir, "", "-updateconfig"); code != 0 || !strings.Contains(out, "as comments") {
		t.Errorf("-updateconfig on a short file: %d %q", code, out)
	}
}

func TestMakeTheme(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := katTasks(t, dir, "", "-maketheme", "mine")
	if code != 0 || !strings.Contains(out, "mine") {
		t.Fatalf("-maketheme: %d %q %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "themes", "mine.toml")); err != nil {
		t.Error(err)
	}
}

func TestRemindFlags(t *testing.T) {
	dir := t.TempDir()
	code, out, _ := katTasks(t, dir, "", "remind", "-h")
	if code != 0 || !strings.Contains(out, "remind [-test") {
		t.Errorf("remind -h: %d %q", code, out)
	}
	if code, _, errOut := katTasks(t, dir, "", "remind", "-bogus"); code != 2 || !strings.Contains(errOut, "bogus") {
		t.Errorf("remind -bogus: %d %q", code, errOut)
	}
	// -test with alerts switched off says so instead of sending nothing.
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[alerts]\npopup = false\nsound = false\n"), 0o644)
	if code, _, errOut := katTasks(t, dir, "", "remind", "-test"); code != 1 || !strings.Contains(errOut, "both off") {
		t.Errorf("remind -test: %d %q", code, errOut)
	}
}
