package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// paletteItem is one row of the command palette.
type paletteItem struct {
	name, desc string
	keys       []string
	disabled   bool
	run        func(m *Model) tea.Cmd
}

func (m *Model) paletteItems() []paletteItem {
	hasTask := m.selected() != nil
	var items []paletteItem
	for _, a := range actions {
		if a.name == "palette" {
			continue
		}
		items = append(items, paletteItem{
			name: a.name, desc: a.desc, keys: m.km.keysFor(a.name),
			disabled: a.needsTask && !hasTask, run: a.run,
		})
	}
	items = append(items, m.savedFilterItems()...)
	items = append(items, m.scriptPaletteItems()...)
	return items
}

func (m *Model) openPalette() {
	m.mode = modePalette
	m.loadScripts()
	m.palItems = m.paletteItems()
	m.palIdx = 0
	m.input.SetValue("")
	m.input.Focus()
	m.filterPalette()
}

// filterPalette ranks the items against the typed query.
func (m *Model) filterPalette() {
	q := strings.ToLower(strings.TrimSpace(m.input.Value()))
	type scored struct {
		i, score int
	}
	var hits []scored
	for i, it := range m.palItems {
		s := max(fuzzyScore(q, it.name), fuzzyScore(q, it.desc)-1)
		if s > 0 {
			hits = append(hits, scored{i, s})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	m.palShown = m.palShown[:0]
	for _, h := range hits {
		m.palShown = append(m.palShown, h.i)
	}
	m.palIdx = clamp(m.palIdx, 0, len(m.palShown)-1)
}

// fuzzyScore rates how well q matches label: a match at the start beats one
// at a word start, which beats one inside a word, which beats a scattered
// subsequence. 0 means no match; an empty query matches everything.
func fuzzyScore(q, label string) int {
	if q == "" {
		return 1
	}
	l := strings.ToLower(label)
	q = strings.ReplaceAll(q, " ", "_")
	l = strings.ReplaceAll(l, " ", "_")
	switch idx := strings.Index(l, q); {
	case idx == 0:
		return 4000 - len(l)
	case idx > 0 && (l[idx-1] == '_' || l[idx-1] == '/' || l[idx-1] == ':'):
		return 3000 - len(l)
	case idx > 0:
		return 2000 - len(l)
	}
	// Subsequence: every query character in order, fewer gaps is better.
	gaps, j := 0, 0
	for i := 0; i < len(l) && j < len(q); i++ {
		if l[i] == q[j] {
			j++
		} else if j > 0 {
			gaps++
		}
	}
	if j < len(q) {
		return 0
	}
	return max(1000-gaps*10-len(l), 1)
}

func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		m.mode = modeNormal
		m.input.Blur()
		if len(m.palShown) == 0 {
			return m, nil
		}
		it := m.palItems[m.palShown[m.palIdx]]
		if it.disabled {
			m.setStatus("%s needs a task under the cursor", it.name)
			return m, nil
		}
		return m, it.run(&m)
	case "down", "ctrl+n", "tab":
		if len(m.palShown) > 0 {
			m.palIdx = (m.palIdx + 1) % len(m.palShown)
		}
		return m, nil
	case "up", "ctrl+p", "shift+tab":
		if len(m.palShown) > 0 {
			m.palIdx = (m.palIdx - 1 + len(m.palShown)) % len(m.palShown)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.palIdx = 0
	m.filterPalette()
	return m, cmd
}

func (m Model) viewPalette(w, h int) string {
	lines := []string{m.st.prompt.Render(": ") + m.input.View(), ""}
	rows := h - 3
	if len(m.palShown) == 0 {
		lines = append(lines, m.st.faint.Render("no matching command"))
		return strings.Join(lines, "\n")
	}
	start := 0
	if m.palIdx >= rows {
		start = m.palIdx - rows + 1
	}
	nameW := 0
	for _, i := range m.palShown {
		nameW = max(nameW, len(m.palItems[i].name))
	}
	nameW = min(nameW, w/3)
	for n := start; n < len(m.palShown) && n < start+rows; n++ {
		it := m.palItems[m.palShown[n]]
		name, desc := m.st.text, m.st.faint
		marker := "  "
		if it.disabled {
			name = m.st.faint
		}
		if n == m.palIdx {
			marker = m.st.prompt.Render("▌ ")
			name = name.Bold(true)
		}
		keys := m.st.helpKey.Render(strings.Join(it.keys, " "))
		left := marker + name.Render(padRight(truncate(it.name, nameW), nameW)) + "  " + desc.Render(it.desc)
		gap := w - 2 - widthOf(left) - widthOf(keys)
		if gap < 1 {
			left = truncateANSI(left, w-2-widthOf(keys)-1)
			gap = 1
		}
		lines = append(lines, left+strings.Repeat(" ", gap)+keys)
	}
	return strings.Join(lines, "\n")
}

func padRight(s string, n int) string {
	if w := len([]rune(s)); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}
