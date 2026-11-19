package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/devinthemtn/katTasks/internal/listview"
)

// clipMsg reports the result of a copy.
type clipMsg struct {
	n   int
	err error
	app bool // copied through the desktop app's clipboard
}

// copyTargets copies the targets' lines (or bodies) to the clipboard: in the
// desktop app through its clipboard; in a terminal always through the OSC 52
// escape, and also through clipboard.command if one is set.
func (m *Model) copyTargets(bodyOnly bool) tea.Cmd {
	ts := m.targets()
	if len(ts) == 0 {
		return nil
	}
	lines := make([]string, len(ts))
	for i, t := range ts {
		if bodyOnly {
			lines[i] = listview.Body(t)
		} else {
			lines[i] = t.String()
		}
	}
	text := strings.Join(lines, "\n")
	m.endVisual()

	n := len(ts)
	if m.copyFn != nil {
		err := m.copyFn(text)
		return func() tea.Msg { return clipMsg{n: n, err: err, app: true} }
	}
	termenv.DefaultOutput().Copy(text)
	cmdline := strings.TrimSpace(m.cfg.Clipboard.Command)
	if cmdline == "" {
		return func() tea.Msg { return clipMsg{n: n} }
	}
	return func() tea.Msg {
		c := exec.Command("sh", "-c", cmdline)
		c.Stdin = strings.NewReader(text)
		var errb bytes.Buffer
		c.Stderr = &errb
		if err := c.Run(); err != nil {
			if e := strings.TrimSpace(errb.String()); e != "" {
				err = fmt.Errorf("%v: %s", err, e)
			}
			return clipMsg{n: n, err: err}
		}
		return clipMsg{n: n}
	}
}

func (m *Model) copied(msg clipMsg) {
	if msg.err != nil && msg.app {
		m.setError("copy failed: %v", msg.err)
		return
	}
	if msg.err != nil {
		m.setError("copied via terminal, but clipboard.command failed: %v", msg.err)
		return
	}
	m.setStatus("copied %s", plural(msg.n, "line"))
}
