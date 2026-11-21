// Package config loads katTasks settings from a commented TOML file, in the
// same style as katdown: the embedded default.toml is the single source of
// defaults, bad values fall back to their default and are reported, and
// -updateconfig adds settings a file is missing as comments.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/devinthemtn/katTasks/internal/atomicfile"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// AppName is the config folder name.
const AppName = "katTasks"

// The text size katTasks-gui allows, in pixels.
const (
	MinFontSize     = 8
	MaxFontSize     = 48
	DefaultFontSize = 14
)

// SettingsFile is the settings file name inside Dir().
const SettingsFile = "config.toml"

// DefaultSettingsFile is the commented settings file written by Make.
//
//go:embed default.toml
var DefaultSettingsFile string

// Config holds the user's settings.
type Config struct {
	Theme string `toml:"theme"`
	Files struct {
		Todo string `toml:"todo"`
		Done string `toml:"done"`
	} `toml:"files"`
	Tasks struct {
		AddCreationDate bool `toml:"add_creation_date"`
		AutoArchive     bool `toml:"auto_archive"`
	} `toml:"tasks"`
	View struct {
		HideCompleted bool     `toml:"hide_completed"`
		Sort          string   `toml:"sort"`
		LineNumbers   bool     `toml:"line_numbers"`
		HideKeys      []string `toml:"hide_keys"`
	} `toml:"view"`
	Clipboard struct {
		Command string `toml:"command"`
	} `toml:"clipboard"`
	GUI struct {
		FontFamily string `toml:"font_family"`
		FontSize   int    `toml:"font_size"`
	} `toml:"gui"`
	Alerts struct {
		Enabled   bool   `toml:"enabled"`
		Lead      string `toml:"lead"`
		Popup     bool   `toml:"popup"`
		Sound     bool   `toml:"sound"`
		SoundFile string `toml:"sound_file"`

		// LeadTime is Lead parsed.
		LeadTime time.Duration `toml:"-"`
	} `toml:"alerts"`
	Alarms struct {
		Enabled     bool   `toml:"enabled"`
		Popup       bool   `toml:"popup"`
		Ring        string `toml:"ring"`
		SoundFile   string `toml:"sound_file"`
		DefaultTime string `toml:"default_time"`

		// RingTime is Ring parsed; DefaultMinutes is DefaultTime as minutes
		// after midnight.
		RingTime       time.Duration `toml:"-"`
		DefaultMinutes int           `toml:"-"`
	} `toml:"alarms"`
	RawKeys map[string]any    `toml:"keys"`
	Filters map[string]string `toml:"filters"`

	// Keys is RawKeys normalized to action -> keys.
	Keys map[string][]string `toml:"-"`
	// Path is the settings file this was loaded from.
	Path string `toml:"-"`

	doneDerived bool // files.done was empty, so it follows files.todo
}

// SetTodo points the config at another todo file (the -file flag). An
// unset files.done moves with it.
func (c *Config) SetTodo(path string) {
	c.Files.Todo = Expand(path)
	if c.doneDerived {
		c.Files.Done = filepath.Join(filepath.Dir(c.Files.Todo), "done.txt")
	}
}

// SortModes are the valid values of view.sort.
var SortModes = []string{"priority", "due", "created", "file"}

// Dir returns the folder holding config.toml and themes/.
func (c Config) Dir() string { return filepath.Dir(c.Path) }

// FilterNames returns the saved search names, sorted.
func (c Config) FilterNames() []string {
	names := make([]string, 0, len(c.Filters))
	for n := range c.Filters {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// DefaultPath is os.UserConfigDir()/katTasks/config.toml, e.g.
// ~/.config/katTasks/config.toml on Linux.
func DefaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, AppName, SettingsFile)
}

// Defaults returns the settings in DefaultSettingsFile.
func Defaults() Config {
	c := rawDefaults()
	c.normalize()
	return c
}

// rawDefaults decodes DefaultSettingsFile without expanding paths, so a
// user's files.todo still decides where an unset files.done goes.
func rawDefaults() Config {
	var c Config
	if _, err := toml.Decode(DefaultSettingsFile, &c); err != nil {
		panic("katTasks: bad default.toml: " + err.Error())
	}
	return c
}

// Load reads the settings at path. A missing file is created from
// DefaultSettingsFile first (created is true). Problems are returned as
// errs, never fatal: a syntax error gives the defaults, and each invalid or
// unknown setting is reported while the rest still apply.
func Load(path string) (c Config, created bool, errs []error) {
	created, err := writeNew(path, DefaultSettingsFile)
	if err != nil {
		errs = append(errs, err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, err)
	}
	c, perrs := Parse(string(data))
	c.Path = path
	return c, created, append(errs, perrs...)
}

// Parse decodes settings from TOML; see Load.
func Parse(data string) (Config, []error) {
	c := rawDefaults()
	meta, err := toml.Decode(data, &c)
	if err != nil {
		d := Defaults()
		return d, []error{fmt.Errorf("%s: %w", SettingsFile, err)}
	}
	var errs []error
	for _, k := range meta.Undecoded() {
		if len(k) > 0 && k[0] == "keys" {
			continue // decoded into RawKeys as any
		}
		errs = append(errs, fmt.Errorf("unknown setting %q", k.String()))
	}
	errs = append(errs, c.normalize()...)
	for i, e := range errs {
		errs[i] = fmt.Errorf("%s: %w", SettingsFile, e)
	}
	return c, errs
}

// normalize expands paths, resets invalid settings to their defaults and
// converts [keys]. It reports what it reset.
func (c *Config) normalize() []error {
	var errs []error
	bad := func(name string, value any, want string) {
		errs = append(errs, fmt.Errorf("%s = %v: %s", name, value, want))
	}

	if strings.TrimSpace(c.Theme) == "" {
		bad("theme", `""`, "want a theme name")
		c.Theme = "default"
	}
	if strings.TrimSpace(c.Files.Todo) == "" {
		bad("files.todo", `""`, "want a file path")
		c.Files.Todo = "~/todo.txt"
	}
	c.Files.Todo = Expand(c.Files.Todo)
	c.doneDerived = c.Files.Done == ""
	if c.doneDerived {
		c.Files.Done = filepath.Join(filepath.Dir(c.Files.Todo), "done.txt")
	} else {
		c.Files.Done = Expand(c.Files.Done)
	}

	valid := false
	for _, s := range SortModes {
		valid = valid || s == c.View.Sort
	}
	if !valid {
		bad("view.sort", fmt.Sprintf("%q", c.View.Sort), `want "priority", "due", "created" or "file"`)
		c.View.Sort = "priority"
	}
	c.GUI.FontFamily = strings.TrimSpace(c.GUI.FontFamily)
	if strings.ContainsAny(c.GUI.FontFamily, ";{}<>\\") {
		bad("gui.font_family", fmt.Sprintf("%q", c.GUI.FontFamily), "want font names separated by commas")
		c.GUI.FontFamily = ""
	}
	if c.GUI.FontSize < MinFontSize || c.GUI.FontSize > MaxFontSize {
		bad("gui.font_size", c.GUI.FontSize, fmt.Sprintf("want %d to %d", MinFontSize, MaxFontSize))
		c.GUI.FontSize = DefaultFontSize
	}
	if d, err := time.ParseDuration(strings.TrimSpace(c.Alerts.Lead)); err != nil || d < 0 || d > 24*time.Hour {
		bad("alerts.lead", fmt.Sprintf("%q", c.Alerts.Lead), `want a time such as "0m", "10m" or "1h"`)
		c.Alerts.Lead, c.Alerts.LeadTime = "10m", 10*time.Minute
	} else {
		c.Alerts.LeadTime = d
	}
	if c.Alerts.SoundFile != "" {
		c.Alerts.SoundFile = Expand(c.Alerts.SoundFile)
	}
	if d, err := time.ParseDuration(strings.TrimSpace(c.Alarms.Ring)); err != nil || d < time.Second || d > 30*time.Minute {
		bad("alarms.ring", fmt.Sprintf("%q", c.Alarms.Ring), `want a time from "1s" to "30m", such as "30s"`)
		c.Alarms.Ring, c.Alarms.RingTime = "30s", 30*time.Second
	} else {
		c.Alarms.RingTime = d
	}
	if c.Alarms.SoundFile != "" {
		c.Alarms.SoundFile = Expand(c.Alarms.SoundFile)
	}
	if min, ok := todo.ParseClock(strings.TrimSpace(c.Alarms.DefaultTime), true); ok {
		c.Alarms.DefaultMinutes = min
	} else {
		bad("alarms.default_time", fmt.Sprintf("%q", c.Alarms.DefaultTime), `want a time such as "7:00" or "6:30am"`)
		c.Alarms.DefaultTime, c.Alarms.DefaultMinutes = "7:00", 7*60
	}
	for i, k := range c.View.HideKeys {
		c.View.HideKeys[i] = strings.ToLower(strings.TrimSpace(k))
	}

	c.Keys = map[string][]string{}
	for action, v := range c.RawKeys {
		switch v := v.(type) {
		case string:
			c.Keys[action] = []string{v}
		case []any:
			var ks []string
			for _, x := range v {
				s, ok := x.(string)
				if !ok {
					bad("keys."+action, v, "want a key or a list of keys")
					ks = nil
					break
				}
				ks = append(ks, s)
			}
			if ks != nil {
				c.Keys[action] = ks
			}
		default:
			bad("keys."+action, v, "want a key or a list of keys")
		}
	}
	if c.Filters == nil {
		c.Filters = map[string]string{}
	}
	return errs
}

// Expand expands a leading ~ and $VARIABLES and makes the path absolute.
func Expand(p string) string {
	p = os.ExpandEnv(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[1:])
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// Make writes DefaultSettingsFile to path unless it already exists.
func Make(path string) (created bool, err error) {
	return writeNew(path, DefaultSettingsFile)
}

// Update adds the settings the file at path is missing as comments; see
// UpdateText. A missing file is created as by Make.
func Update(path string) (added []string, created bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		created, err = Make(path)
		return nil, created, err
	}
	if err != nil {
		return nil, false, err
	}
	out, added, err := UpdateText(string(data))
	if err != nil || len(added) == 0 {
		return nil, false, err
	}
	return added, false, replaceFile(path, out)
}

// Save sets one value in the settings file at path, keeping everything else
// (including comments) as it is; see SetValue. value is TOML, e.g. `"nord"`.
func Save(path, table, key, value string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		data, err = []byte(DefaultSettingsFile), nil
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	return replaceFile(path, SetValue(string(data), table, key, value))
}

// writeNew writes data to path unless the file already exists.
func writeNew(path, data string) (created bool, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(data); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}

// replaceFile replaces the settings file at path with data, whole or not
// at all; see atomicfile.
func replaceFile(path, data string) error {
	return atomicfile.Write(path, []byte(data), 0o644)
}
