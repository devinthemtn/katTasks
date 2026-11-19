package alert

import (
	"strings"
)

const (
	runKey   = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "katTasks remind" // the Windows installer uses the same value
)

// Install adds a Run entry, which starts katTasks remind at login with no
// console window (conhost --headless).
func Install(args []string) (string, error) {
	exe, err := executable()
	if err != nil {
		return "", err
	}
	cmd := []string{"conhost.exe", "--headless", winQuote(exe)}
	for _, a := range args {
		cmd = append(cmd, winQuote(a))
	}
	if err := run("reg.exe", []string{"add", runKey, "/v", runValue, "/t", "REG_SZ", "/d", strings.Join(cmd, " "), "/f"}, nil, ""); err != nil {
		return "", err
	}
	return `added "` + runValue + `" to ` + runKey + `; it starts at your next sign-in`, nil
}

// Uninstall removes the Run entry.
func Uninstall() (string, error) {
	run("reg.exe", []string{"delete", runKey, "/v", runValue, "/f"}, nil, "") // fine if missing
	return `removed "` + runValue + `" from ` + runKey, nil
}

func winQuote(s string) string {
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
