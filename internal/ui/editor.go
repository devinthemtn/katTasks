package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devinthemtn/katTasks/internal/config"
)

// editorDoneMsg reports that the editor opened by openInEditor has closed.
type editorDoneMsg struct {
	path string
	err  error
}

// editorVar is the environment variable naming the editor: $EDITOR in the
// terminal, where katTasks steps aside while it runs, and $VISUAL in
// katTasks-gui, which has no terminal to lend it.
func (m Model) editorVar() string {
	if m.windowed {
		return "VISUAL"
	}
	return "EDITOR"
}

// settingsPath is the settings file in use.
func (m Model) settingsPath() string {
	if m.opts.ConfigPath != "" {
		return m.opts.ConfigPath
	}
	return config.DefaultPath()
}

// openInEditor opens path (a file or folder) in the user's editor, and
// re-reads the settings once it closes. With no editor set, it hands path
// to the system's default app instead.
func (m *Model) openInEditor(path string) tea.Cmd {
	ed := strings.TrimSpace(os.Getenv(m.editorVar()))
	if ed == "" {
		if err := systemOpen(path); err != nil {
			m.setError("$%s isn't set, and opening %s failed: %v", m.editorVar(), path, err)
		} else {
			m.setStatus("opened %s ($%s isn't set) — ctrl+r reloads the settings", path, m.editorVar())
		}
		return nil
	}
	c := editorCommand(ed, path)
	if m.windowed {
		m.setStatus("opened %s in %s", path, ed)
		return func() tea.Msg { return editorDoneMsg{path, c.Run()} }
	}
	return tea.ExecProcess(c, func(err error) tea.Msg { return editorDoneMsg{path, err} })
}

// systemOpen opens path with the system's default app for it.
func systemOpen(path string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		c = exec.Command("open", path)
	default:
		c = exec.Command("xdg-open", path)
	}
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait() //nolint:errcheck // reap it; the app reports its own errors
	return nil
}

// editorDone re-reads the settings after the editor closes, since they or
// a theme in their folder may have changed.
func (m *Model) editorDone(msg editorDoneMsg) {
	if msg.err != nil {
		m.setError("editor: %v", msg.err)
		return
	}
	m.reloadConfig()
}

// editSettings opens the settings file; editConfigDir opens its folder.
func (m *Model) editSettings() tea.Cmd { return m.openInEditor(m.settingsPath()) }

func (m *Model) editConfigDir() tea.Cmd {
	dir := filepath.Dir(m.settingsPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.setError("settings folder: %v", err)
		return nil
	}
	return m.openInEditor(dir)
}
