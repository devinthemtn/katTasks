package alert

import "errors"

const macSound = "Glass"

// send uses Notification Center through osascript (the text goes in as
// arguments, never as AppleScript), and afplay for a sound file.
func send(title, body string, s Settings) error {
	var errs []error
	if s.Popup {
		script := `on run argv
display notification (item 2 of argv) with title (item 1 of argv)`
		if s.Sound && s.SoundFile == "" {
			script += ` sound name "` + macSound + `"`
		}
		errs = append(errs, run("osascript", []string{"-e", script + "\nend run", title, body}, nil, ""))
	}
	switch {
	case s.Sound && s.SoundFile != "":
		errs = append(errs, run("afplay", []string{s.SoundFile}, nil, ""))
	case s.Sound && !s.Popup:
		errs = append(errs, run("afplay", []string{"/System/Library/Sounds/" + macSound + ".aiff"}, nil, ""))
	}
	return errors.Join(errs...)
}
