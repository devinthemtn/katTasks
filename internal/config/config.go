// Package config loads katTasks settings from a TOML file.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds user settings.
type Config struct {
	TodoFile        string `toml:"todo_file"`
	DoneFile        string `toml:"done_file"`
	AddCreationDate bool   `toml:"add_creation_date"`
	AutoArchive     bool   `toml:"auto_archive"`
	HideCompleted   bool   `toml:"hide_completed"`
	DefaultSort     string `toml:"default_sort"`
}

const defaultFile = `# katTasks configuration

# Path to your todo.txt file. "~" expands to your home directory.
todo_file = "~/todo.txt"

# Where archived (completed) tasks go. Empty means done.txt next to todo_file.
done_file = ""

# Prefix new tasks with today's date.
add_creation_date = true

# Move tasks to done_file as soon as they are completed.
auto_archive = false

# Hide completed tasks when katTasks starts (toggle with "H").
hide_completed = false

# Initial sort: "priority", "due", "created" or "file".
default_sort = "priority"
`

// DefaultPath is $XDG_CONFIG_HOME/katTasks/config.toml, falling back to
// ~/.config/katTasks/config.toml.
func DefaultPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "katTasks", "config.toml")
}

// Load reads the config at path. If the file does not exist, a commented
// default config is written there first. created reports whether that
// happened.
func Load(path string) (cfg Config, created bool, err error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return cfg, false, err
		}
		if err := os.WriteFile(path, []byte(defaultFile), 0o644); err != nil {
			return cfg, false, err
		}
		created = true
	}
	if _, err := toml.Decode(defaultFile, &cfg); err != nil {
		return cfg, created, err
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, created, err
	}
	cfg.TodoFile = expand(cfg.TodoFile)
	if cfg.DoneFile == "" {
		cfg.DoneFile = filepath.Join(filepath.Dir(cfg.TodoFile), "done.txt")
	} else {
		cfg.DoneFile = expand(cfg.DoneFile)
	}
	switch cfg.DefaultSort {
	case "priority", "due", "created", "file":
	default:
		cfg.DefaultSort = "priority"
	}
	return cfg, created, nil
}

func expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[1:])
	}
	if abs, err := filepath.Abs(os.ExpandEnv(p)); err == nil {
		return abs
	}
	return p
}
