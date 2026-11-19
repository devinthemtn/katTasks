package alert

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const launchLabel = "com.devinthemtn.katTasks.remind"

func plistFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchLabel+".plist")
}

// Install writes a launch agent that runs at login, and starts it now.
func Install(args []string) (string, error) {
	exe, err := executable()
	if err != nil {
		return "", err
	}
	var argv strings.Builder
	for _, a := range append([]string{exe}, args...) {
		argv.WriteString("\t\t<string>")
		xml.EscapeText(&argv, []byte(a))
		argv.WriteString("</string>\n")
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchLabel + `</string>
	<key>ProgramArguments</key>
	<array>
` + argv.String() + `	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
</dict>
</plist>
`
	f := plistFile()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return "", err
	}
	run("launchctl", []string{"unload", f}, nil, "") // an older copy; fine if none
	if err := os.WriteFile(f, []byte(plist), 0o644); err != nil {
		return "", err
	}
	if err := run("launchctl", []string{"load", "-w", f}, nil, ""); err != nil {
		return "", err
	}
	return "added " + f + " and started it; it also starts at login", nil
}

// Uninstall stops the launch agent and removes it.
func Uninstall() (string, error) {
	f := plistFile()
	run("launchctl", []string{"unload", f}, nil, "")
	if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return "stopped and removed " + f, nil
}
