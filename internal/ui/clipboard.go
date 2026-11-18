package ui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// clipMsg reports the result of a copy.
type clipMsg struct {
	n   int
	err error
}

// body is a task's text without priority, dates, +projects, @contexts and
// key:value tags.
func body(t *todo.Task) string {
	var out []string
	for _, w := range strings.Fields(t.Text) {
		if (len(w) > 1 && (w[0] == '+' || w[0] == '@')) || isTag(w) {
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// copyTargets copies the targets' lines (or bodies) to the clipboard: always
// through the terminal's OSC 52 escape, and also through clipboard.command
// if one is set.
func (m *Model) copyTargets(bodyOnly bool) tea.Cmd {
	ts := m.targets()
	if len(ts) == 0 {
		return nil
	}
	lines := make([]string, len(ts))
	for i, t := range ts {
		if bodyOnly {
			lines[i] = body(t)
		} else {
			lines[i] = t.String()
		}
	}
	text := strings.Join(lines, "\n")
	m.endVisual()

	termenv.DefaultOutput().Copy(text)
	cmdline := strings.TrimSpace(m.cfg.Clipboard.Command)
	n := len(ts)
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
	if msg.err != nil {
		m.setError("copied via terminal, but clipboard.command failed: %v", msg.err)
		return
	}
	m.setStatus("copied %s", plural(msg.n, "line"))
}
