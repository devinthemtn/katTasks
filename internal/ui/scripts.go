package ui

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/script"
	"github.com/devinthemtn/katTasks/internal/todo"
)

const maxScriptSuggestions = 5

// loadScripts rescans the scripts folder, so new files show up without a
// restart.
func (m *Model) loadScripts() {
	m.scripts = script.List(script.Dir(m.cfg.Dir()))
}

func (m *Model) findScript(name string) (script.Script, bool) {
	for _, s := range m.scripts {
		if s.Name == name {
			return s, true
		}
	}
	return script.Script{}, false
}

// scriptPaletteItems are the : entries for the scripts.
func (m *Model) scriptPaletteItems() []paletteItem {
	var items []paletteItem
	for _, s := range m.scripts {
		desc := s.Desc
		if desc == "" {
			desc = "lua script"
		}
		items = append(items, paletteItem{
			name: "script:" + s.Name, desc: desc,
			run: func(m *Model) tea.Cmd { m.runScriptFromPalette(s); return nil },
		})
	}
	return items
}

// scriptSuggestions lists /name completions for a word starting with "/".
func (m *Model) scriptSuggestions(w string) []string {
	q := strings.ToLower(w[1:])
	type scored struct {
		name  string
		score int
	}
	var hits []scored
	for _, s := range m.scripts {
		if sc := fuzzyScore(q, s.Name); sc > 0 {
			hits = append(hits, scored{s.Name, sc})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	var out []string
	for _, h := range hits {
		out = append(out, "/"+h.name)
		if len(out) == maxScriptSuggestions {
			break
		}
	}
	return out
}

// isScriptSuggestion reports whether the suggestions are /script names.
func (m *Model) isScriptSuggestion() bool {
	return len(m.sugg) > 0 && strings.HasPrefix(m.sugg[0], "/")
}

func (m *Model) scriptEnv(inline bool, input string) script.Env {
	cur := m.selected()
	if inline {
		cur = m.editing
	}
	return script.Env{Inline: inline, Input: input, Current: cur, Tasks: m.tasks, Now: time.Now()}
}

// runInlineScript replaces the word at [start, end) of the input, which
// names s, with what s returns. It reports whether the script ran.
func (m *Model) runInlineScript(s script.Script, start, end int) bool {
	val := []rune(m.input.Value())
	before, after := string(val[:start]), string(val[end:])
	rest := strings.Join(strings.Fields(before+" "+after), " ")
	res, err := script.Run(s, m.scriptEnv(true, rest))
	if err != nil {
		m.setError("%v", err)
		return false
	}
	out := strings.TrimSpace(strings.ReplaceAll(res.Output, "\n", " "))
	var nv string
	var cur int
	if out == "" {
		// Drop the word and one of the spaces around it.
		b := strings.TrimRight(before, " ")
		a := strings.TrimLeft(after, " ")
		sep := ""
		if b != "" && a != "" {
			sep = " "
		}
		nv, cur = b+sep+a, len([]rune(b+sep))
	} else {
		if after == "" || after[0] != ' ' {
			out += " "
		}
		nv, cur = before+out+after, len([]rune(before+out))
	}
	m.input.SetValue(nv)
	m.input.SetCursor(cur)
	m.applyScriptResult(s, res, false)
	return true
}

// expandScripts runs every /name word that names a script, before the
// task is saved. It reports false if a script failed.
func (m *Model) expandScripts() bool {
	for range 32 { // a script could return another /name; don't loop forever
		val := []rune(m.input.Value())
		found := false
		for i := 0; i < len(val); {
			if val[i] == ' ' {
				i++
				continue
			}
			j := i
			for j < len(val) && val[j] != ' ' {
				j++
			}
			if w := string(val[i:j]); len(w) > 1 && w[0] == '/' {
				if s, ok := m.findScript(w[1:]); ok {
					if !m.runInlineScript(s, i, j) {
						return false
					}
					found = true
					break
				}
			}
			i = j
		}
		if !found {
			return true
		}
	}
	return true
}

func (m *Model) runScriptFromPalette(s script.Script) {
	res, err := script.Run(s, m.scriptEnv(false, ""))
	if err != nil {
		m.setError("%v", err)
		return
	}
	m.applyScriptResult(s, res, true)
}

// applyScriptResult makes the changes a script asked for, as one undo step.
func (m *Model) applyScriptResult(s script.Script, res script.Result, fromPalette bool) {
	now := time.Now()
	var added []*todo.Task
	for _, line := range res.Adds {
		added = append(added, todo.NewTask(line, now, m.cfg.Tasks.AddCreationDate))
	}
	target := m.selected()
	var upd *todo.Task
	if res.Update != nil && target != nil {
		upd = todo.Parse(*res.Update)
		upd.NormalizeDue(now)
	}
	changed := len(added) > 0 || upd != nil
	if changed {
		ok := m.mutate(func() {
			m.tasks = append(m.tasks, added...)
			if upd != nil {
				*target = *upd
			}
		})
		if !ok {
			return
		}
		m.refresh(target)
	}

	msg := res.Status
	if msg == "" && fromPalette && res.HasOutput {
		msg = res.Output
	}
	switch {
	case msg != "":
		m.setStatus("%s", msg)
	case fromPalette && len(added) > 0 && upd != nil:
		m.setStatus("%s: added %d, updated 1", s.Name, len(added))
	case len(added) > 0:
		m.setStatus("%s: added %d", s.Name, len(added))
	case upd != nil:
		m.setStatus("%s: updated", s.Name)
	case fromPalette:
		m.setStatus("ran %s", s.Name)
	}
}

func (m *Model) scriptDesc(name string) string {
	if s, ok := m.findScript(strings.TrimPrefix(name, "/")); ok {
		return s.Desc
	}
	return ""
}
