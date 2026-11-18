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

	"github.com/BurntSushi/toml"
)

// AppName is the config folder name.
const AppName = "katTasks"

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

// replaceFile writes data to path through a temporary file, so a failed
// write leaves the old file intact. The file keeps its permissions.
func replaceFile(path, data string) error {
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), SettingsFile+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
