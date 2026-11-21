//go:build !darwin && !windows

package alert

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/todo"
)

type call struct {
	name string
	args []string
}

// fakeOS pretends that only the given programs are installed, records
// what would run instead of running it, and fails the programs in fail.
func fakeOS(t *testing.T, wsl bool, installed []string, fail ...string) *[]call {
	t.Helper()
	var mu sync.Mutex
	var calls []call
	oldRun, oldLook, oldWSL := run, lookPath, isWSL
	t.Cleanup(func() { run, lookPath, isWSL = oldRun, oldLook, oldWSL })
	run = func(name string, args, env []string, dir string) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, call{name, args})
		if slices.Contains(fail, name) {
			return errors.New(name + " failed")
		}
		return nil
	}
	lookPath = func(name string) (string, error) {
		if slices.Contains(installed, name) {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	isWSL = func() bool { return wsl }
	return &calls
}

func names(calls []call) []string {
	var out []string
	for _, c := range calls {
		out = append(out, c.name)
	}
	return out
}

func TestPopupUsesNotifySend(t *testing.T) {
	calls := fakeOS(t, false, []string{"notify-send", "gdbus"})
	if err := popup("Title", "Body", true); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.name != "notify-send" || !slices.Contains(c.args, "critical") || c.args[len(c.args)-2] != "Title" || c.args[len(c.args)-1] != "Body" {
		t.Errorf("ran %s %q", c.name, c.args)
	}
}

func TestPopupFallsBackToGdbus(t *testing.T) {
	calls := fakeOS(t, false, []string{"gdbus"})
	if err := popup(`Say "hi"`, "Body", false); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.name != "gdbus" || !slices.Contains(c.args, "org.freedesktop.Notifications.Notify") || !slices.Contains(c.args, `"Say \"hi\""`) {
		t.Errorf("ran %s %q", c.name, c.args)
	}
}

func TestPopupWithNothingInstalled(t *testing.T) {
	fakeOS(t, false, nil)
	if err := popup("t", "b", false); err == nil || !strings.Contains(err.Error(), "install") {
		t.Errorf("error %v, want a hint to install something", err)
	}
}

func TestSoundPrograms(t *testing.T) {
	calls := fakeOS(t, false, []string{"canberra-gtk-play", "paplay"})
	sound("")
	sound("/tmp/ding.oga")
	if got := names(*calls); !slices.Equal(got, []string{"canberra-gtk-play", "paplay"}) {
		t.Errorf("ran %v", got)
	}
	if args := (*calls)[1].args; !slices.Equal(args, []string{"/tmp/ding.oga"}) {
		t.Errorf("paplay %q", args)
	}

	fakeOS(t, false, nil)
	if err := sound("/tmp/ding.oga"); err == nil {
		t.Error("no player installed, but no error")
	}
}

// A popup that fails doesn't stop the sound, and both errors are reported.
func TestSendDoesBothParts(t *testing.T) {
	calls := fakeOS(t, false, []string{"notify-send", "paplay"}, "notify-send")
	err := send("t", "b", Settings{Popup: true, Sound: true, SoundFile: "/tmp/ding.oga"})
	if err == nil || !strings.Contains(err.Error(), "notify-send failed") {
		t.Errorf("error %v", err)
	}
	if got := names(*calls); !slices.Equal(got, []string{"notify-send", "paplay"}) {
		t.Errorf("ran %v", got)
	}
}

func TestSendRingsUntilDone(t *testing.T) {
	calls := fakeOS(t, false, []string{"paplay"})
	start := time.Now()
	if err := send("t", "b", Settings{Sound: true, SoundFile: "/tmp/ding.oga", Ring: 30 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 30*time.Millisecond || len(*calls) < 2 {
		t.Errorf("rang %d times in %v", len(*calls), time.Since(start))
	}
}

func TestRingStopsOnError(t *testing.T) {
	n := 0
	err := ring(time.Hour, func() error { n++; return errors.New("no sound") })
	if err == nil || n != 1 {
		t.Errorf("played %d times, err %v", n, err)
	}
}

// Under WSL, alerts go to Windows through PowerShell.
func TestSendUnderWSL(t *testing.T) {
	calls := fakeOS(t, true, []string{"notify-send"})
	send("t", "b", Settings{Popup: true})
	if got := names(*calls); len(got) != 1 || got[0] != "powershell.exe" {
		t.Errorf("ran %v", got)
	}
}

func TestInstallAndUninstall(t *testing.T) {
	fakeOS(t, false, nil)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	msg, err := Install([]string{"remind", "-file", "/home/me/My Tasks/todo.txt"})
	if err != nil {
		t.Fatal(err)
	}
	f := autostartFile()
	b, err := os.ReadFile(f)
	if err != nil || !strings.Contains(msg, f) {
		t.Fatalf("message %q, file %v", msg, err)
	}
	if !strings.Contains(string(b), ` remind -file "/home/me/My Tasks/todo.txt"`+"\n") || !strings.Contains(string(b), "Type=Application") {
		t.Errorf("entry:\n%s", b)
	}
	if filepath.Dir(f) != filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart") {
		t.Errorf("entry in %s", f)
	}

	for i := 0; i < 2; i++ { // removing it twice is fine
		if _, err := Uninstall(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Errorf("entry still there (%v)", err)
	}
}

func TestInstallUnderWSL(t *testing.T) {
	fakeOS(t, true, nil)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Install([]string{"remind"}); err == nil || !strings.Contains(err.Error(), "Windows") {
		t.Errorf("error %v", err)
	}
	if _, err := os.Stat(autostartFile()); !os.IsNotExist(err) {
		t.Error("entry written under WSL")
	}
}

func TestDesktopQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin/katTasks": "/usr/bin/katTasks",
		"/a b/c":            `"/a b/c"`,
		`say "hi"`:          `"say \\"hi\\""`,
		"$HOME":             `"\\$HOME"`,
		`back\slash`:        `"back\\\\slash"`,
	} {
		if got := desktopQuote(in); got != want {
			t.Errorf("desktopQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// Watch checks at once and then every tick, logs an error once while it
// keeps happening, and stops when its context ends.
func TestWatch(t *testing.T) {
	var mu sync.Mutex
	checks, logs := 0, 0
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Watch(ctx, Options{
			Dir:   t.TempDir(),
			Every: 5 * time.Millisecond,
			Source: func() (config.Config, []*todo.Task, error) {
				mu.Lock()
				defer mu.Unlock()
				checks++
				return config.Config{}, nil, errors.New("todo.txt unreadable")
			},
			Logf: func(string, ...any) { mu.Lock(); logs++; mu.Unlock() },
		})
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch didn't stop when its context ended")
	}
	mu.Lock()
	defer mu.Unlock()
	if checks < 3 || logs != 1 {
		t.Errorf("%d checks, %d logs; want several checks and the repeated error logged once", checks, logs)
	}
}
