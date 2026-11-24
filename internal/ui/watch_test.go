package ui

import (
	"strings"
	"testing"
	"time"
)

func TestWatchReloadsChangedFile(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one", "two", "three")
	m.cfg.Files.WatchEvery = time.Minute
	m, cmd := update(m, watchStartMsg{})
	if cmd == nil {
		t.Fatal("no timer started")
	}
	m = onTask(t, m, "two")
	editExternally(t, m, "zero\none\ntwo\nthree\n")

	m, cmd = update(m, watchMsg{m.watchSeq})
	if cmd == nil {
		t.Error("timer not restarted")
	}
	if len(m.tasks) != 4 || m.selected().Text != "two" {
		t.Fatalf("after reload: %d tasks, cursor on %v", len(m.tasks), m.selected())
	}
	if m.stale() || !strings.Contains(m.status, "reloaded") {
		t.Errorf("stale %v, status %q", m.stale(), m.status)
	}

	// An old timer's tick is ignored.
	editExternally(t, m, "only\n")
	if m, _ = update(m, watchMsg{m.watchSeq - 1}); len(m.tasks) != 4 {
		t.Error("an old timer reloaded the file")
	}
}

func TestWatchWaitsWhileTyping(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one")
	m.cfg.Files.WatchEvery = time.Minute
	m, _ = update(m, watchStartMsg{})
	m = send(t, m, "a")
	editExternally(t, m, "one\ntwo\n")
	m, cmd := update(m, watchMsg{m.watchSeq})
	if len(m.tasks) != 1 || m.mode != modeAdd || cmd == nil {
		t.Fatalf("reloaded while typing: %d tasks, mode %v", len(m.tasks), m.mode)
	}
}

func TestWatchOff(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one")
	m.cfg.Files.WatchEvery = 0
	if _, cmd := update(m, watchStartMsg{}); cmd != nil {
		t.Error("timer started with files.watch = false")
	}
}

func TestSyncAction(t *testing.T) {
	m := fileModel(t, fileOpts{}, "one")
	m.cfg.Files.WatchEvery = 0
	actionByName["sync"].run(&m)
	if !strings.Contains(m.status, "up to date") {
		t.Errorf("status %q", m.status)
	}
	editExternally(t, m, "one\ntwo\n")
	actionByName["sync"].run(&m)
	if len(m.tasks) != 2 || !strings.Contains(m.status, "reloaded") {
		t.Errorf("%d tasks, status %q", len(m.tasks), m.status)
	}
}
