package alert

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Send delivers a: a desktop notification if s.Popup, and a sound if
// s.Sound. It returns once the sound has played. A part that fails doesn't
// stop the other.
func Send(a Alert, s Settings) error {
	title, body := a.Message()
	return send(title, body, s)
}

// Test sends a sample alert, for katTasks remind -test.
func Test(s Settings) error {
	now := time.Now()
	return Send(Alert{Text: "This is how a katTasks reminder looks", At: now}, s)
}

// run runs a command, and turns a failure into an error that includes what
// it printed.
func run(name string, args []string, env []string, dir string) error {
	c := exec.Command(name, args...)
	c.Env, c.Dir = env, dir
	hide(c)
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		if msg != "" {
			return fmt.Errorf("%s: %v: %s", name, err, msg)
		}
		return fmt.Errorf("%s: %v", name, err)
	}
	return nil
}

// ring plays a sound with play over and over until d has passed (at least
// once), for alarms.
func ring(d time.Duration, play func() error) error {
	end := time.Now().Add(d)
	for {
		if err := play(); err != nil {
			return err
		}
		if !time.Now().Before(end) {
			return nil
		}
	}
}

// TestAlarm rings a sample alarm, for katTasks remind -test-alarm.
func TestAlarm(s Settings) error {
	now := time.Now()
	return Send(Alert{Text: "This is how a katTasks alarm sounds", At: now, Alarm: true}, s)
}

// first returns the first of the programs that is on the PATH.
func first(names ...string) string {
	for _, n := range names {
		if _, err := exec.LookPath(n); err == nil {
			return n
		}
	}
	return ""
}
