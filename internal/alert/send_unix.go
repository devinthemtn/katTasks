//go:build !darwin && !windows

package alert

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// freedesktopSound is the notification sound most Linux desktops ship.
const freedesktopSound = "/usr/share/sounds/freedesktop/stereo/message.oga"

// send uses the desktop's notification service (notify-send, or D-Bus
// through gdbus) and PulseAudio, PipeWire or ALSA for sound. Under WSL it
// sends a Windows notification instead.
func send(title, body string, s Settings) error {
	if isWSL() {
		return sendPowerShell(title, body, s, true)
	}
	var errs []error
	if s.Popup {
		errs = append(errs, popup(title, body))
	}
	if s.Sound {
		errs = append(errs, sound(s.SoundFile))
	}
	return errors.Join(errs...)
}

func isWSL() bool {
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}

func popup(title, body string) error {
	switch first("notify-send", "gdbus") {
	case "notify-send":
		return run("notify-send", []string{"-a", "katTasks", "-i", "appointment-soon", title, body}, nil, "")
	case "gdbus":
		return run("gdbus", []string{"call", "--session",
			"--dest", "org.freedesktop.Notifications", "--object-path", "/org/freedesktop/Notifications",
			"--method", "org.freedesktop.Notifications.Notify",
			`"katTasks"`, "0", `"appointment-soon"`, strconv.Quote(title), strconv.Quote(body), "[]", "{}", "-1"}, nil, "")
	}
	return errors.New("no notify-send or gdbus for notifications (install libnotify-bin or libnotify)")
}

func sound(file string) error {
	if file == "" {
		if first("canberra-gtk-play") != "" {
			return run("canberra-gtk-play", []string{"-i", "message-new-instant", "-d", "katTasks"}, nil, "")
		}
		if _, err := os.Stat(freedesktopSound); err != nil {
			return errors.New("no sound to play: install canberra-gtk-play (libcanberra) or set alerts.sound_file")
		}
		file = freedesktopSound
	}
	p := first("paplay", "pw-play", "aplay")
	if p == "" {
		return errors.New("no paplay, pw-play or aplay to play the alert sound")
	}
	return run(p, []string{file}, nil, "")
}
