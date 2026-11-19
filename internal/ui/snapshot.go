package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devinthemtn/katTasks/internal/listview"
	"github.com/devinthemtn/katTasks/internal/theme"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// Snapshot is the screen as data, for katTasks-gui to draw. It holds what
// View draws, in the same order and with the same rules, but as rows and
// styled pieces of text instead of terminal escapes.
type Snapshot struct {
	Seq int `json:"seq"` // increases with every snapshot, so a late one can be dropped

	Mode    string `json:"mode"`  // normal, add, edit, filter, priority, confirm, theme, palette, savename
	Focus   string `json:"focus"` // list or sidebar
	Version string `json:"version"`

	Path string `json:"path"` // todo.txt, with ~ for the home folder
	Info string `json:"info"` // "3 pending · sort: priority · …"

	Side []SideRow `json:"side"` // the sidebar rows on screen

	Rows   []TaskRow `json:"rows"`   // the task rows on screen
	Total  int       `json:"total"`  // tasks in the list
	Offset int       `json:"offset"` // index of Rows[0]
	Empty  string    `json:"empty"`  // shown when the list is empty
	NumW   int       `json:"numW"`   // width of line numbers; 0 = off

	Input   *Input    `json:"input"`   // the text being typed, if any
	Confirm string    `json:"confirm"` // the y/n question, in confirm mode
	Footer  [2][]Seg  `json:"footer"`  // the two status lines
	Visual  bool      `json:"visual"`  // visual (multi-select) mode
	Help    []HelpCol `json:"help"`    // the help screen, when open

	Calendar *Calendar `json:"calendar"` // shown instead of the list, when open
	Agenda   *Agenda   `json:"agenda"`   // the list as a week, when open

	Palette []PaletteRow `json:"palette"` // matching commands, in palette mode
	PalIdx  int          `json:"palIdx"`

	Themes  []ThemeRow `json:"themes"` // in theme mode
	PickIdx int        `json:"pickIdx"`
	Theme   string     `json:"theme"`
	Colors  Colors     `json:"colors"`

	Font     string `json:"font"`     // CSS font-family, from gui.font_family
	FontSize int    `json:"fontSize"` // pixels, from gui.font_size
}

// Seg is a piece of styled text. Style is a theme color key, or one of
// "key" (a key name), "chip" (a highlighted label), "prompt", "ok", "error".
// Action, if set, is the action the piece names, so it can be clicked.
type Seg struct {
	Text   string `json:"text"`
	Style  string `json:"style,omitempty"`
	Action string `json:"action,omitempty"`
}

// SideRow is one sidebar row.
type SideRow struct {
	Index  int    `json:"index"`
	Kind   string `json:"kind"` // see listview.Kind
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Cursor bool   `json:"cursor"`
}

// TaskRow is one task line.
type TaskRow struct {
	Index    int    `json:"index"` // in the list
	Num      int    `json:"num"`   // line in todo.txt
	Done     bool   `json:"done"`
	Priority string `json:"priority"`
	PriStyle string `json:"priStyle"`
	Words    []Word `json:"words"`
	Due      string `json:"due"`
	DueStyle string `json:"dueStyle"`
	Recur    bool   `json:"recur"`
	// Status and Progress are the in-progress badges, e.g. "▶ doing" and
	// "▰▰▱▱▱ 40%"; empty for none. Doing makes the status bright.
	Status   string `json:"status"`
	Doing    bool   `json:"doing"`
	Progress string `json:"progress"`
	Cursor   bool   `json:"cursor"`
	Marked   bool   `json:"marked"`
}

// Word is one word of a task, with its theme color and whether it matches
// the search.
type Word struct {
	Text  string `json:"text"`
	Style string `json:"style"`
	Match bool   `json:"match"`
}

// Input is a line being typed.
type Input struct {
	Label    string   `json:"label"` // "New task", "filter", …
	Value    string   `json:"value"`
	Pos      int      `json:"pos"` // cursor, in runes
	Sugg     []string `json:"sugg"`
	SuggIdx  int      `json:"suggIdx"`
	SuggDesc string   `json:"suggDesc"` // what the chosen /script does
	Hint     string   `json:"hint"`
}

// HelpCol is a column of the help screen.
type HelpCol struct {
	Title string    `json:"title"`
	Rows  []HelpRow `json:"rows"`
}

// HelpRow is one action and its keys.
type HelpRow struct {
	Keys   string `json:"keys"`
	Desc   string `json:"desc"`
	Action string `json:"action"`
}

// PaletteRow is one command palette entry.
type PaletteRow struct {
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Keys     string `json:"keys"`
	Disabled bool   `json:"disabled"`
}

// ThemeRow is one theme in the picker.
type ThemeRow struct {
	Name string `json:"name"`
	File string `json:"file"` // themes/x.toml, or "" for a built-in
}

// Colors are a theme's colors as CSS hex, for light and dark windows: the
// theme's color keys plus "bg", the window background. "" is the default.
type Colors struct {
	Light map[string]string `json:"light"`
	Dark  map[string]string `json:"dark"`
}

var modeNames = map[mode]string{
	modeNormal: "normal", modeAdd: "add", modeEdit: "edit", modeFilter: "filter", modePriority: "priority",
	modeConfirm: "confirm", modeTheme: "theme", modePalette: "palette", modeSaveName: "savename",
}

// Snapshot returns the screen as data.
func (m Model) Snapshot() Snapshot {
	s := Snapshot{
		Mode: modeNames[m.mode], Focus: "list", Version: m.opts.Version,
		Total: len(m.visible), Offset: m.offset, Visual: m.visual,
		PalIdx: m.palIdx, PickIdx: m.pickIdx, Theme: m.theme,
	}
	if m.focus == focusSidebar {
		s.Focus = "sidebar"
	}
	s.Path, s.Info = m.headerParts()

	rows := m.listRows()
	for i := m.sideOffset; i < len(m.side) && i < m.sideOffset+rows; i++ {
		e := m.side[i]
		s.Side = append(s.Side, SideRow{Index: i, Kind: e.Kind.String(), Label: e.Label(), Count: e.Count, Cursor: i == m.sideCursor})
	}

	s.Rows, s.NumW = m.taskRows(rows)
	if len(m.visible) == 0 {
		s.Empty = "No tasks. Press a to add one."
		if m.filter != "" || m.sideCursor != 0 {
			s.Empty = "No matching tasks. Press esc to clear the filter."
		}
	}

	switch m.mode {
	case modeAdd, modeEdit, modeFilter, modeSaveName, modePalette:
		s.Input = m.inputState()
	case modeConfirm:
		s.Confirm = m.confirm
	case modeTheme:
		for _, t := range m.themes {
			r := ThemeRow{Name: t.Name}
			if t.File != "" {
				r.File = filepath.Join("themes", filepath.Base(t.File))
			}
			s.Themes = append(s.Themes, r)
		}
	}
	if m.mode == modePalette {
		for _, i := range m.palShown {
			it := m.palItems[i]
			s.Palette = append(s.Palette, PaletteRow{Name: it.name, Desc: it.desc, Keys: strings.Join(it.keys, " "), Disabled: it.disabled})
		}
	}
	if m.showHelp && m.mode == modeNormal {
		s.Help = m.helpCols()
	}
	if m.calendar {
		s.Calendar = m.calendarSnapshot()
	}
	if m.agenda {
		s.Agenda = m.agendaSnapshot()
	}
	s.Footer = m.footerSegs()
	s.Font, s.FontSize = cssFontFamily(m.cfg.GUI.FontFamily), m.cfg.GUI.FontSize
	if t, ok := theme.Find(m.themes, m.theme); ok {
		s.Colors = cssColors(t)
	}
	return s
}

func (m Model) headerParts() (path, info string) {
	pending := 0
	for _, t := range m.tasks {
		if !t.Done {
			pending++
		}
	}
	parts := []string{fmt.Sprintf("%d pending", pending), "sort: " + sortModes[m.sortIdx]}
	if m.hideDone {
		parts = append(parts, "done hidden")
	}
	if m.filter != "" && m.mode != modeFilter {
		parts = append(parts, fmt.Sprintf("filter: %q", m.filter))
	}
	path = m.cfg.Files.Todo
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		path = "~" + path[len(home):]
	}
	return path, strings.Join(parts, " · ")
}

// taskRows mirrors viewList and renderTask.
func (m Model) taskRows(rows int) ([]TaskRow, int) {
	var nums map[*todo.Task]int
	numW := 0
	if m.lineNums {
		nums = make(map[*todo.Task]int, len(m.tasks))
		for i, t := range m.tasks {
			nums[t] = i + 1
		}
		numW = len(fmt.Sprint(len(m.tasks)))
	}
	hide := m.hiddenKeys()
	now := time.Now()
	var out []TaskRow
	for i := m.offset; i < len(m.visible) && i < m.offset+rows; i++ {
		r := m.taskRow(m.visible[i], hide, now)
		r.Index, r.Num, r.Cursor = i, nums[m.visible[i]], i == m.cursor
		out = append(out, r)
	}
	return out, numW
}

// taskRow mirrors renderTask for one task.
func (m Model) taskRow(t *todo.Task, hide map[string]bool, now time.Time) TaskRow {
	r := TaskRow{Done: t.Done, Marked: m.sel[t]}
	if t.Priority != 0 {
		r.Priority = string(t.Priority)
		r.PriStyle = priKey(t.Priority)
	}
	r.Status, r.Doing, r.Progress = listview.WorkLabels(t)
	_, r.Recur = t.Recurrence()
	r.Recur = r.Recur && !t.Done
	d, hasDue := t.Due()
	if hasDue {
		label, kind := listview.DueLabel(d, now)
		r.Due, r.DueStyle = label, dueKey(kind)
		if t.Done {
			r.DueStyle = "dim"
		}
	}
	for _, word := range strings.Fields(t.Text) {
		st := "fg"
		switch {
		case strings.HasPrefix(word, "due:") && hasDue:
			continue
		case !t.Done && todo.IsWorkTag(word): // shown as a badge
			continue
		case t.Done:
			st = "done"
		case len(word) > 1 && word[0] == '+':
			st = "project"
		case len(word) > 1 && word[0] == '@':
			st = "context"
		case isTag(word):
			if k, _, _ := strings.Cut(word, ":"); hide[strings.ToLower(k)] {
				continue
			}
			st = "tag"
		}
		r.Words = append(r.Words, Word{Text: word, Style: st, Match: !t.Done && m.q.Highlights(word)})
	}
	return r
}

// hiddenKeys are the tags view.hide_keys leaves off rows.
func (m Model) hiddenKeys() map[string]bool {
	hide := map[string]bool{}
	for _, k := range m.cfg.View.HideKeys {
		hide[k] = true
	}
	return hide
}

func priKey(p byte) string {
	if p >= 'A' && p <= 'D' {
		return "pri_" + strings.ToLower(string(p))
	}
	return "pri_other"
}

func dueKey(k listview.DueKind) string {
	switch k {
	case listview.DueOverdue:
		return "overdue"
	case listview.DueToday:
		return "today"
	case listview.DueSoon:
		return "due"
	}
	return "dim"
}

func (m Model) inputState() *Input {
	in := &Input{Value: m.input.Value(), Pos: m.input.Position(), SuggIdx: m.suggIdx}
	switch m.mode {
	case modeAdd:
		in.Label = "New task"
	case modeEdit:
		in.Label = "Edit task"
	case modeFilter:
		in.Label = "filter"
		in.Hint = "+proj @ctx due:+1w due:-3d due:overdue pri:A-C -word \"a phrase\" · enter keep · esc clear"
	case modeSaveName:
		in.Label = "save search as"
		in.Hint = "saves " + m.currentQuery() + " to [filters] in " + m.opts.ConfigPath
	case modePalette:
		in.Label = ":"
		return in
	}
	in.Sugg = m.sugg
	if m.isScriptSuggestion() && m.suggIdx < len(m.sugg) {
		in.SuggDesc = m.scriptDesc(m.sugg[m.suggIdx])
	}
	return in
}

// footerSegs mirrors viewFooter, for the modes whose text isn't drawn
// elsewhere in the window.
func (m Model) footerSegs() [2][]Seg {
	var l1, l2 []Seg
	switch m.mode {
	case modeAdd, modeEdit:
		l1 = []Seg{{Text: map[mode]string{modeAdd: "add task", modeEdit: "edit task"}[m.mode], Style: "prompt"}}
		l2 = []Seg{{Text: "enter save · esc cancel", Style: "dim"}}
	case modeFilter, modeSaveName:
		// The input and its hint are drawn in the footer by the app.
	case modeConfirm:
		l1 = []Seg{{Text: m.confirm, Style: "prompt"}}
	case modePriority:
		l1 = []Seg{{Text: "priority › ", Style: "prompt"}, {Text: "press A–Z, space to clear, esc to cancel", Style: "dim"}}
	case modeTheme:
		l1 = []Seg{{Text: "theme › ", Style: "prompt"}, {Text: m.theme, Style: "fg"}}
	case modePalette:
		l1 = []Seg{{Text: "type to search commands · ↑↓ choose · enter run · esc close", Style: "dim"}}
	default:
		switch {
		case m.status != "" && m.statusErr:
			l1 = []Seg{{Text: m.status, Style: "error"}}
		case m.status != "":
			l1 = []Seg{{Text: m.status, Style: "ok"}}
		case m.configWarn != "":
			l1 = []Seg{{Text: "⚠ " + m.configWarn, Style: "error"}, {Text: "  (fix, then ctrl+r)", Style: "dim"}}
		}
		if m.calendar {
			l2 = []Seg{{Text: calendarHelp, Style: "dim"}}
		} else if m.agenda && !m.visual {
			l2 = []Seg{{Text: agendaHelp, Style: "dim"}}
		} else if m.visual {
			l2 = []Seg{{Text: fmt.Sprintf("VISUAL %d", len(m.sel)), Style: "chip"},
				{Text: " space mark · x done · d delete · p priority · yy copy · esc leave", Style: "dim"}}
		} else {
			for _, name := range shortHelpActions {
				ks := m.km.keysFor(name)
				if len(ks) == 0 {
					continue
				}
				if len(l2) > 0 {
					l2 = append(l2, Seg{Text: " • ", Style: "dim"})
				}
				l2 = append(l2, Seg{Text: ks[0], Style: "key", Action: name}, Seg{Text: " " + shortLabel(name), Style: "dim", Action: name})
			}
		}
		if m.pending != "" {
			l2 = append([]Seg{{Text: displayKey(m.pending) + "…", Style: "chip"}, {Text: " "}}, l2...)
		}
	}
	return [2][]Seg{l1, l2}
}

// helpCols mirrors fullHelp.
func (m Model) helpCols() []HelpCol {
	var cols []HelpCol
	for _, g := range []string{groupMove, groupEdit, groupView} {
		c := HelpCol{Title: g}
		for _, a := range actions {
			if a.group != g {
				continue
			}
			if ks := m.km.keysFor(a.name); len(ks) > 0 {
				c.Rows = append(c.Rows, HelpRow{Keys: strings.Join(ks, " "), Desc: a.desc, Action: a.name})
			}
		}
		cols = append(cols, c)
	}
	return cols
}

// ---- colors --------------------------------------------------------------------

// cssColors resolves a theme for light and dark windows. Terminal themes
// have no background color, so one is chosen: the background a built-in dark
// theme was made for, the usual white or near-black for themes that adapt,
// and a shade of the cursor row for other fixed dark ones.
func cssColors(t theme.Theme) Colors {
	c := Colors{Light: map[string]string{}, Dark: map[string]string{}}
	for _, f := range theme.Fields {
		col := t.Get(f.Key)
		c.Light[f.Key], c.Dark[f.Key] = cssColor(col.Light), cssColor(col.Dark)
	}
	c.Light["bg"], c.Dark["bg"] = "#FFFFFF", "#18181F"
	if bg, ok := builtinBg[t.Name]; ok && t.File == "" {
		c.Light["bg"], c.Dark["bg"] = bg, bg
		return c
	}
	fg := t.Get("fg")
	if fg.Light == fg.Dark && fg.Dark != "" {
		if lum(cssColor(fg.Dark)) > 0.5 {
			bg := shade(cssColor(t.Get("cursor_bg").Dark), 0.75)
			if bg == "" {
				bg = "#18181F"
			}
			c.Light["bg"], c.Dark["bg"] = bg, bg
		} else {
			c.Dark["bg"] = "#FFFFFF"
		}
	}
	return c
}

// builtinBg are the backgrounds the built-in dark themes were designed on.
var builtinBg = map[string]string{"nord": "#2E3440", "gruvbox": "#282828", "dracula": "#282A36"}

// monoFonts back up gui.font_family, and are the family when it is empty.
var monoFonts = []string{"JetBrains Mono", "Cascadia Mono", "Cascadia Code", "Consolas", "DejaVu Sans Mono", "Liberation Mono", "Menlo", "monospace"}

var genericFonts = map[string]bool{"monospace": true, "ui-monospace": true, "sans-serif": true, "serif": true, "system-ui": true, "ui-sans-serif": true}

// cssFontFamily turns gui.font_family ("Fira Code, Menlo") into a CSS font-family
// list: each name quoted, followed by the monospace fonts.
func cssFontFamily(font string) string {
	var out []string
	seen := map[string]bool{}
	for _, n := range append(strings.Split(font, ","), monoFonts...) {
		n = strings.Trim(strings.TrimSpace(n), `"'`)
		if n == "" || seen[strings.ToLower(n)] {
			continue
		}
		seen[strings.ToLower(n)] = true
		if genericFonts[strings.ToLower(n)] {
			out = append(out, strings.ToLower(n))
		} else {
			out = append(out, `"`+strings.ReplaceAll(n, `"`, "")+`"`)
		}
	}
	return strings.Join(out, ", ")
}

// ansi16 are xterm's first sixteen colors.
var ansi16 = [16]string{
	"#000000", "#CD0000", "#00CD00", "#CDCD00", "#0000EE", "#CD00CD", "#00CDCD", "#E5E5E5",
	"#7F7F7F", "#FF0000", "#00FF00", "#FFFF00", "#5C5CFF", "#FF00FF", "#00FFFF", "#FFFFFF",
}

// cssColor turns a theme color ("#rgb", "#rrggbb" or ANSI "0"–"255") into
// "#RRGGBB"; "" stays "".
func cssColor(s string) string {
	if s == "" {
		return ""
	}
	if s[0] == '#' {
		if len(s) == 4 {
			return strings.ToUpper("#" + s[1:2] + s[1:2] + s[2:3] + s[2:3] + s[3:4] + s[3:4])
		}
		return strings.ToUpper(s)
	}
	n, err := strconv.Atoi(s)
	switch {
	case err != nil || n < 0 || n > 255:
		return ""
	case n < 16:
		return ansi16[n]
	case n >= 232:
		v := 8 + (n-232)*10
		return fmt.Sprintf("#%02X%02X%02X", v, v, v)
	}
	n -= 16
	level := func(i int) int {
		if i == 0 {
			return 0
		}
		return 55 + i*40
	}
	return fmt.Sprintf("#%02X%02X%02X", level(n/36), level(n/6%6), level(n%6))
}

func rgb(hex string) (r, g, b float64, ok bool) {
	if len(hex) != 7 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(v >> 16 & 0xFF), float64(v >> 8 & 0xFF), float64(v & 0xFF), true
}

// lum is a color's rough brightness, 0–1.
func lum(hex string) float64 {
	r, g, b, ok := rgb(hex)
	if !ok {
		return 0
	}
	return (0.299*r + 0.587*g + 0.114*b) / 255
}

// shade scales a color toward black.
func shade(hex string, f float64) string {
	r, g, b, ok := rgb(hex)
	if !ok {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X", int(r*f), int(g*f), int(b*f))
}
