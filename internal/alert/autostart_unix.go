//go:build !darwin && !windows

package alert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The XDG autostart entry, which GNOME, KDE, Xfce and most other desktops
// start at login.
func autostartFile() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "katTasks-remind.desktop")
}

// Install writes the autostart entry.
func Install(args []string) (string, error) {
	if isWSL() {
		return "", errors.New("in WSL, run katTasks.exe remind -install on the Windows side; the Windows installer also sets it up")
	}
	exe, err := executable()
	if err != nil {
		return "", err
	}
	cmd := []string{desktopQuote(exe)}
	for _, a := range args {
		cmd = append(cmd, desktopQuote(a))
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=katTasks reminders
Comment=Alerts for katTasks tasks at their at: time
Exec=%s
Terminal=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
`, strings.Join(cmd, " "))
	f := autostartFile()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(f, []byte(entry), 0o644); err != nil {
		return "", err
	}
	return "added " + f + "; it starts at your next login", nil
}

// Uninstall removes it.
func Uninstall() (string, error) {
	f := autostartFile()
	if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return "removed " + f, nil
}

// desktopQuote quotes an Exec= argument as the desktop entry spec asks.
func desktopQuote(s string) string {
	if !strings.ContainsAny(s, " \t\"'\\$`") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", "$", `\\$`)
	return `"` + r.Replace(s) + `"`
}
