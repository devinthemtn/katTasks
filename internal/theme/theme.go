// Package theme defines katTasks color themes: a few built in, plus any
// themes/*.toml next to the settings file.
package theme

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Color is a terminal color. Light and Dark differ for colors that adapt to
// the terminal's background; both empty means the terminal's own color.
// Values are "#rgb", "#rrggbb" or an ANSI number "0"–"255".
type Color struct{ Light, Dark string }

// Plain returns a color that is the same on light and dark terminals.
func Plain(c string) Color { return Color{c, c} }

func (c Color) adaptive() bool { return c.Light != c.Dark }

// Field describes one themable color.
type Field struct{ Key, Desc string }

// Fields lists every color a theme sets, in file order.
var Fields = []Field{
	{"fg", "task text"},
	{"dim", "secondary text: counts, hints, tags, far-off due dates"},
	{"accent", "prompts, cursor marker, suggestions"},
	{"border", "pane borders"},
	{"border_focus", "border of the focused pane"},
	{"cursor_bg", "background of the row under the cursor"},
	{"selected_bg", "background of rows selected in visual mode"},
	{"match_bg", "background of words matching the search"},
	{"title_fg", "katTasks title text"},
	{"title_bg", "katTasks title background"},
	{"project", "+project"},
	{"context", "@context"},
	{"tag", "key:value tags"},
	{"pri_a", "priority A"},
	{"pri_b", "priority B"},
	{"pri_c", "priority C"},
	{"pri_d", "priority D"},
	{"pri_other", "priorities E–Z"},
	{"due", "due within a week"},
	{"today", "due today"},
	{"overdue", "past due"},
	{"done", "completed tasks"},
	{"ok", "success messages, done checkbox"},
	{"error", "error messages"},
}

// Theme is a named set of colors, keyed by Field.Key.
type Theme struct {
	Name   string
	Colors map[string]Color
	File   string // "" for built-ins
}

// Get returns a color, or the terminal default if unset.
func (t Theme) Get(key string) Color { return t.Colors[key] }

var colorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func validColor(s string) bool {
	if s == "" || colorRe.MatchString(s) {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
}

func ad(light, dark string) Color { return Color{light, dark} }

// Builtins are the themes that ship with katTasks, in picker order.
var Builtins = []Theme{
	{Name: "default", Colors: map[string]Color{
		"fg": ad("#1F1F24", "#E6E6EA"), "dim": ad("#8A8A8A", "#6E6E7A"), "accent": ad("#6D4AE8", "#A78BFA"),
		"border": ad("#8A8A8A", "#6E6E7A"), "border_focus": ad("#6D4AE8", "#A78BFA"),
		"cursor_bg": ad("#E9E4FF", "#34304A"), "selected_bg": ad("#FFF1C2", "#4A4128"), "match_bg": ad("#FFE58F", "#5C4A00"),
		"title_fg": Plain("#FFFFFF"), "title_bg": ad("#6D4AE8", "#7C5CE0"),
		"project": ad("#2E7D32", "#86D99A"), "context": ad("#0B6FA4", "#7CC4FA"), "tag": ad("#8A8A8A", "#6E6E7A"),
		"pri_a": ad("#C62828", "#FF7A7A"), "pri_b": ad("#D35400", "#FFB062"), "pri_c": ad("#9A7B00", "#F2D46B"),
		"pri_d": ad("#546E7A", "#9FB3C8"), "pri_other": ad("#546E7A", "#9FB3C8"),
		"due": ad("#9A7B00", "#F2D46B"), "today": ad("#D35400", "#FFB062"), "overdue": ad("#C62828", "#FF7A7A"),
		"done": ad("#8A8A8A", "#6E6E7A"), "ok": ad("#2E7D32", "#86D99A"), "error": ad("#C62828", "#FF7A7A"),
	}},
	{Name: "terminal", Colors: plain(map[string]string{
		"fg": "", "dim": "8", "accent": "5", "border": "8", "border_focus": "5",
		"cursor_bg": "8", "selected_bg": "4", "match_bg": "3", "title_fg": "15", "title_bg": "5",
		"project": "2", "context": "6", "tag": "8", "pri_a": "1", "pri_b": "3", "pri_c": "4", "pri_d": "5", "pri_other": "8",
		"due": "3", "today": "11", "overdue": "9", "done": "8", "ok": "2", "error": "1",
	})},
	{Name: "nord", Colors: plain(map[string]string{
		"fg": "#D8DEE9", "dim": "#616E88", "accent": "#88C0D0", "border": "#4C566A", "border_focus": "#88C0D0",
		"cursor_bg": "#3B4252", "selected_bg": "#434C5E", "match_bg": "#5E81AC", "title_fg": "#2E3440", "title_bg": "#88C0D0",
		"project": "#A3BE8C", "context": "#81A1C1", "tag": "#616E88",
		"pri_a": "#BF616A", "pri_b": "#D08770", "pri_c": "#EBCB8B", "pri_d": "#B48EAD", "pri_other": "#8FBCBB",
		"due": "#EBCB8B", "today": "#D08770", "overdue": "#BF616A", "done": "#616E88", "ok": "#A3BE8C", "error": "#BF616A",
	})},
	{Name: "gruvbox", Colors: plain(map[string]string{
		"fg": "#EBDBB2", "dim": "#928374", "accent": "#FABD2F", "border": "#504945", "border_focus": "#FABD2F",
		"cursor_bg": "#3C3836", "selected_bg": "#504945", "match_bg": "#665C54", "title_fg": "#282828", "title_bg": "#FABD2F",
		"project": "#B8BB26", "context": "#83A598", "tag": "#928374",
		"pri_a": "#FB4934", "pri_b": "#FE8019", "pri_c": "#FABD2F", "pri_d": "#D3869B", "pri_other": "#8EC07C",
		"due": "#FABD2F", "today": "#FE8019", "overdue": "#FB4934", "done": "#928374", "ok": "#B8BB26", "error": "#FB4934",
	})},
	// Dracula: https://draculatheme.com (MIT).
	{Name: "dracula", Colors: plain(map[string]string{
		"fg": "#F8F8F2", "dim": "#6272A4", "accent": "#BD93F9", "border": "#44475A", "border_focus": "#BD93F9",
		"cursor_bg": "#44475A", "selected_bg": "#4B3F66", "match_bg": "#6B5F2A", "title_fg": "#282A36", "title_bg": "#BD93F9",
		"project": "#50FA7B", "context": "#8BE9FD", "tag": "#6272A4",
		"pri_a": "#FF5555", "pri_b": "#FFB86C", "pri_c": "#F1FA8C", "pri_d": "#FF79C6", "pri_other": "#8BE9FD",
		"due": "#F1FA8C", "today": "#FFB86C", "overdue": "#FF5555", "done": "#6272A4", "ok": "#50FA7B", "error": "#FF5555",
	})},
}

func plain(m map[string]string) map[string]Color {
	out := make(map[string]Color, len(m))
	for k, v := range m {
		out[k] = Plain(v)
	}
	return out
}

// Dir is the themes folder inside the settings folder.
func Dir(configDir string) string { return filepath.Join(configDir, "themes") }

// fileTheme is a theme file as decoded.
type fileTheme struct {
	name, base, path string
	raw              map[string]any
}

// Load returns the built-in themes followed by those in themes/ (sorted by
// file name). A file with the same name as a built-in replaces it. Files
// that can't be used are skipped and reported in warnings.
func Load(configDir string) (themes []Theme, warnings []error) {
	files := map[string]*fileTheme{}
	var order []string
	entries, err := os.ReadDir(Dir(configDir))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		warnings = append(warnings, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		p := filepath.Join(Dir(configDir), e.Name())
		var raw map[string]any
		if _, err := toml.DecodeFile(p, &raw); err != nil {
			warnings = append(warnings, fmt.Errorf("theme %s: %w", e.Name(), err))
			continue
		}
		ft := &fileTheme{name: strings.TrimSuffix(e.Name(), ".toml"), path: p, raw: raw, base: "default"}
		if b, ok := raw["base"].(string); ok {
			ft.base = b
		}
		files[ft.name] = ft
		order = append(order, ft.name)
	}
	sort.Strings(order)

	builtin := map[string]Theme{}
	for _, b := range Builtins {
		builtin[b.Name] = b
	}
	resolved := map[string]Theme{}
	var resolve func(name string, seen map[string]bool) (Theme, error)
	resolve = func(name string, seen map[string]bool) (Theme, error) {
		if t, ok := resolved[name]; ok {
			return t, nil
		}
		ft, ok := files[name]
		if !ok {
			if b, ok := builtin[name]; ok {
				return b, nil
			}
			return Theme{}, fmt.Errorf("unknown base theme %q", name)
		}
		if seen[name] {
			return Theme{}, fmt.Errorf("base themes loop through %q", name)
		}
		seen[name] = true
		var base Theme
		var err error
		if ft.base == name {
			b, ok := builtin[name]
			if !ok {
				return Theme{}, fmt.Errorf("theme %q can't be its own base", name)
			}
			base = b
		} else if base, err = resolve(ft.base, seen); err != nil {
			return Theme{}, err
		}
		t, err := decode(ft, base)
		if err != nil {
			return Theme{}, err
		}
		resolved[name] = t
		return t, nil
	}

	for _, b := range Builtins {
		if _, overridden := files[b.Name]; !overridden {
			themes = append(themes, b)
		}
	}
	for _, name := range order {
		t, err := resolve(name, map[string]bool{})
		if err != nil {
			warnings = append(warnings, fmt.Errorf("theme %s.toml: %w", name, err))
			if b, ok := builtin[name]; ok {
				themes = append(themes, b) // keep the built-in it failed to replace
			}
			continue
		}
		themes = append(themes, t)
	}
	return themes, warnings
}

func decode(ft *fileTheme, base Theme) (Theme, error) {
	t := Theme{Name: ft.name, File: ft.path, Colors: map[string]Color{}}
	for k, v := range base.Colors {
		t.Colors[k] = v
	}
	known := map[string]bool{"base": true, "name": true}
	for _, f := range Fields {
		known[f.Key] = true
	}
	var bad []string
	for k, v := range ft.raw {
		if !known[k] {
			bad = append(bad, fmt.Sprintf("unknown color %q", k))
			continue
		}
		if k == "base" || k == "name" {
			continue
		}
		c, err := parseColor(v)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: %v", k, err))
			continue
		}
		t.Colors[k] = c
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return Theme{}, errors.New(strings.Join(bad, "; "))
	}
	return t, nil
}

func parseColor(v any) (Color, error) {
	switch v := v.(type) {
	case string:
		if !validColor(v) {
			return Color{}, fmt.Errorf("%q is not a color (want \"#rrggbb\", \"0\"–\"255\" or \"\")", v)
		}
		return Plain(v), nil
	case map[string]any:
		l, lok := v["light"].(string)
		d, dok := v["dark"].(string)
		if !lok || !dok || len(v) != 2 {
			return Color{}, fmt.Errorf("want { light = \"…\", dark = \"…\" }")
		}
		if !validColor(l) || !validColor(d) {
			return Color{}, fmt.Errorf("light/dark %q / %q: not colors", l, d)
		}
		return Color{l, d}, nil
	}
	return Color{}, fmt.Errorf("want a color string or { light, dark }")
}

// Find returns the theme with the given name.
func Find(themes []Theme, name string) (Theme, bool) {
	for _, t := range themes {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}

// Format renders t as a complete, commented theme file.
func Format(t Theme) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# katTasks theme, copied from %q.\n#\n", t.Name)
	b.WriteString("# Colors are \"#rrggbb\", an ANSI number \"0\"-\"255\", \"\" for the terminal's\n")
	b.WriteString("# own color, or { light = \"…\", dark = \"…\" } to follow the terminal background.\n")
	b.WriteString("# Leave a color out to take it from the base theme.\n\n")
	b.WriteString("base = \"default\"\n")
	for _, f := range Fields {
		c := t.Get(f.Key)
		val := strconv.Quote(c.Dark)
		if c.adaptive() {
			val = fmt.Sprintf("{ light = %q, dark = %q }", c.Light, c.Dark)
		}
		fmt.Fprintf(&b, "\n# %s\n%s = %s\n", f.Desc, f.Key, val)
	}
	return b.String()
}

// Write saves t as configDir/themes/name.toml, refusing to overwrite.
func Write(configDir, name string, t Theme) (string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("bad theme name %q", name)
	}
	p := filepath.Join(Dir(configDir), name+".toml")
	if err := os.MkdirAll(Dir(configDir), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return p, fmt.Errorf("%s already exists; left unchanged", p)
	}
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(Format(t)); err != nil {
		f.Close()
		return "", err
	}
	return p, f.Close()
}
