// Package alert reminds you of tasks at their at: time, with a desktop
// notification and a sound, on Linux, macOS and Windows.
//
// katTasks, katTasks-gui and the katTasks remind background command all run
// Watch. They share a folder of markers, one per alert, created atomically,
// so whichever process gets there first sends it and the others skip it.
package alert

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// CatchUp is how late an alert may still go off, e.g. after the computer
// wakes from sleep. Older ones are skipped.
const CatchUp = 15 * time.Minute

// keepMarkers is how long a sent alert's marker is kept.
const keepMarkers = 48 * time.Hour

// Alert is one reminder.
type Alert struct {
	Text   string    // the task, without its tags
	At     time.Time // the task's at: time on its day
	FireAt time.Time // At minus the lead
	Alarm  bool      // an alarm: tag, rather than an at: reminder
}

// Message is the notification's title and body.
func (a Alert) Message() (title, body string) {
	if a.Alarm {
		return "⏰ katTasks alarm", a.At.Format("Mon Jan 2 ") + todo.FormatClock(a.At.Hour()*60+a.At.Minute()) + "  " + a.Text
	}
	return "katTasks", todo.FormatClock(a.At.Hour()*60+a.At.Minute()) + "  " + a.Text
}

// Settings say how an alert is delivered.
type Settings struct {
	Popup     bool
	Sound     bool
	SoundFile string // empty: the system's notification sound

	// Ring, for alarms, plays SoundFile over and over for this long.
	Ring time.Duration
	// Urgent asks the desktop to keep the notification up (alarms).
	Urgent bool
}

// SettingsOf reads the [alerts] settings.
func SettingsOf(c config.Config) Settings {
	return Settings{Popup: c.Alerts.Popup, Sound: c.Alerts.Sound, SoundFile: c.Alerts.SoundFile}
}

// AlarmSettingsOf reads the [alarms] settings. Without a sound_file it
// rings with the built-in alarm.
func AlarmSettingsOf(c config.Config) (Settings, error) {
	s := Settings{Popup: c.Alarms.Popup, Sound: true, SoundFile: c.Alarms.SoundFile, Ring: c.Alarms.RingTime, Urgent: true}
	if s.SoundFile == "" {
		f, err := AlarmSoundFile()
		if err != nil {
			return s, err
		}
		s.SoundFile = f
	}
	return s, nil
}

// Pending returns the alerts that should go off at now: open tasks with an
// at: tag, on their due: day (or today without one), whose at: time minus
// lead has passed by less than CatchUp. Times written loosely in a task's
// text ("call 3pm") don't count, only at: tags.
func Pending(tasks []*todo.Task, now time.Time, lead time.Duration) []Alert {
	var out []Alert
	for _, t := range tasks {
		if t.Done {
			continue
		}
		v, ok := t.Tag("at")
		if !ok {
			continue
		}
		min, ok := todo.ParseClock(v, true)
		if !ok {
			continue
		}
		day, ok := t.Due()
		if !ok {
			y, m, d := now.Date()
			day = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		}
		at := time.Date(day.Year(), day.Month(), day.Day(), min/60, min%60, 0, 0, now.Location())
		fire := at.Add(-lead)
		if now.Before(fire) || !now.Before(fire.Add(CatchUp)) {
			continue
		}
		text := listview.Body(t)
		if text == "" {
			text = t.Text
		}
		out = append(out, Alert{Text: text, At: at, FireAt: fire})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// Alarms returns the alarms that should ring at now: open tasks whose
// alarm: time has passed by less than CatchUp.
func Alarms(tasks []*todo.Task, now time.Time) []Alert {
	var out []Alert
	for _, t := range tasks {
		at, ok := t.Alarm()
		if t.Done || !ok || now.Before(at) || !now.Before(at.Add(CatchUp)) {
			continue
		}
		text := listview.Body(t)
		if text == "" {
			text = t.Text
		}
		out = append(out, Alert{Text: text, At: at, FireAt: at, Alarm: true})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// MarkerDir is where sent alerts are recorded: the user cache folder plus
// katTasks/alerted.
func MarkerDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, config.AppName, "alerted")
}

// key names an alert's marker. It uses the at: time, not the lead, so
// changing alerts.lead doesn't repeat an alert.
func key(a Alert) string {
	kind := "at"
	if a.Alarm {
		kind = "alarm"
	}
	h := sha1.Sum([]byte(kind + "\x00" + a.At.Format(time.RFC3339) + "\x00" + a.Text))
	return hex.EncodeToString(h[:])
}

// Claim records that a is being sent, and reports whether this caller is
// the first to do so. Creating the marker is atomic, so of several
// processes only one gets true.
func Claim(dir string, a Alert) (bool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(filepath.Join(dir, key(a)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fmt.Fprintln(f, a.At.Format(time.RFC3339), a.Text)
	return true, f.Close()
}

// prune deletes markers older than keepMarkers.
func prune(dir string, now time.Time) {
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > keepMarkers {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Source returns the current settings and tasks.
type Source func() (config.Config, []*todo.Task, error)

// FileSource reads the settings file and the todo file it names (or
// todoFile, the -file flag, when set), rereading each only when it changes.
func FileSource(cfgPath, todoFile string) Source {
	var (
		cfg              config.Config
		cfgTime, tasksAt time.Time
		tasksPath        string
		tasks            []*todo.Task
		loaded           bool
	)
	return func() (config.Config, []*todo.Task, error) {
		if mt := todo.ModTime(cfgPath); !loaded || !mt.Equal(cfgTime) {
			cfg, _, _ = config.Load(cfgPath) // bad settings fall back to defaults
			if todoFile != "" {
				cfg.SetTodo(todoFile)
			}
			cfgTime, loaded = mt, true
		}
		if mt := todo.ModTime(cfg.Files.Todo); cfg.Files.Todo != tasksPath || !mt.Equal(tasksAt) {
			ts, _, err := todo.Load(cfg.Files.Todo)
			if err != nil {
				return cfg, nil, err
			}
			tasks, tasksPath, tasksAt = ts, cfg.Files.Todo, mt
		}
		return cfg, tasks, nil
	}
}

// Options configure Watch. Only Source is required.
type Options struct {
	Source Source
	Send   func(Alert, Settings) error // default Send
	Dir    string                      // markers; default MarkerDir()
	Now    func() time.Time            // default time.Now
	Every  time.Duration               // how often to look; default 30s
	Logf   func(format string, a ...any)
}

func (o *Options) defaults() {
	if o.Send == nil {
		o.Send = Send
	}
	if o.Dir == "" {
		o.Dir = MarkerDir()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Every == 0 {
		o.Every = 30 * time.Second
	}
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
}

// Watch checks for alerts now and then every o.Every until ctx is done.
// Errors go to o.Logf, each distinct one once in a row.
func Watch(ctx context.Context, o Options) {
	o.defaults()
	last := ""
	logErr := func(err error) {
		if err == nil {
			last = ""
			return
		}
		if msg := err.Error(); msg != last {
			last = msg
			o.Logf("alerts: %v", err)
		}
	}
	tick := time.NewTicker(o.Every)
	defer tick.Stop()
	for {
		logErr(o.Check())
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Check sends the alerts and rings the alarms that are due now; see Watch.
// An alarm rings for alarms.ring before Check returns.
func (o Options) Check() error {
	o.defaults()
	cfg, tasks, err := o.Source()
	if err != nil {
		return err
	}
	now := o.Now()
	prune(o.Dir, now)
	var errs []error
	send := func(as []Alert, s Settings) {
		for _, a := range as {
			first, err := Claim(o.Dir, a)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if first {
				errs = append(errs, o.Send(a, s))
			}
		}
	}
	if cfg.Alarms.Enabled {
		if as := Alarms(tasks, now); len(as) > 0 {
			s, err := AlarmSettingsOf(cfg)
			errs = append(errs, err)
			send(as, s)
		}
	}
	if s := SettingsOf(cfg); cfg.Alerts.Enabled && (s.Popup || s.Sound) {
		send(Pending(tasks, now, cfg.Alerts.LeadTime), s)
	}
	return errors.Join(errs...)
}
