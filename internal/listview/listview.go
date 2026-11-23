// Package listview holds the task-list logic both katTasks frontends share:
// the terminal UI and the katTasks-gui desktop app. It decides which tasks a
// sidebar entry shows, how the list is sorted, what the sidebar contains and
// how a due date is labeled, so the two look and behave the same.
package listview

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// SortModes are the list orders, in the order the sort key cycles them.
var SortModes = config.SortModes

// Kind is the type of a sidebar entry.
type Kind int

const (
	All     Kind = iota // every task
	Due                 // tasks with a due date
	Header              // a section title, not selectable
	Project             // one +project
	Context             // one @context
	Saved               // a saved search from [filters]
	Linked              // tasks linked to a katnotes note (kn:)
)

// String is the kind's name, as the GUI sends it.
func (k Kind) String() string {
	return [...]string{"all", "due", "header", "project", "context", "saved", "linked"}[k]
}

// Entry is one row of the sidebar.
type Entry struct {
	Kind  Kind
	Value string      // project, context, section title or saved search name
	Q     query.Query // Saved
	Count int         // open tasks it shows
}

// Label is the text shown for the entry.
func (e Entry) Label() string {
	switch e.Kind {
	case All:
		return "All tasks"
	case Due:
		return "Has due date"
	case Linked:
		return "Linked to notes"
	}
	return e.Value
}

// Match reports whether the entry shows t.
func (e Entry) Match(t *todo.Task, now time.Time) bool {
	switch e.Kind {
	case Due:
		_, ok := t.Due()
		return ok
	case Linked:
		_, ok := LinkedNote(t)
		return ok
	case Project, Context:
		return t.Has(e.Value)
	case Saved:
		return e.Q.Match(t, now)
	}
	return true
}

// Sidebar builds the sidebar for tasks: All, Due, then SAVED searches,
// PROJECTS and CONTEXTS, each with its count of open tasks. known lists
// every +project and @context, most used first, for completion.
func Sidebar(tasks []*todo.Task, filters map[string]string, now time.Time) (side []Entry, known []string) {
	projects, contexts := map[string]int{}, map[string]int{}
	used := map[string]int{}
	due, pending, linked := 0, 0, 0
	count := func(set map[string]int, w string, done bool) {
		used[w]++
		if _, ok := set[w]; !ok {
			set[w] = 0
		}
		if !done {
			set[w]++
		}
	}
	for _, t := range tasks {
		for _, p := range t.Projects() {
			count(projects, p, t.Done)
		}
		for _, c := range t.Contexts() {
			count(contexts, c, t.Done)
		}
		if t.Done {
			continue
		}
		pending++
		if _, ok := t.Due(); ok {
			due++
		}
	}
	anyLinked := false
	for _, t := range tasks {
		if _, ok := LinkedNote(t); ok {
			anyLinked = true
			if !t.Done {
				linked++
			}
		}
	}

	side = []Entry{{Kind: All, Count: pending}, {Kind: Due, Count: due}}
	if anyLinked {
		side = append(side, Entry{Kind: Linked, Count: linked})
	}
	if len(filters) > 0 {
		names := make([]string, 0, len(filters))
		for n := range filters {
			names = append(names, n)
		}
		sort.Strings(names)
		side = append(side, Entry{Kind: Header, Value: "SAVED"})
		for _, n := range names {
			e := Entry{Kind: Saved, Value: n, Q: query.Parse(filters[n])}
			for _, t := range tasks {
				if !t.Done && e.Q.Match(t, now) {
					e.Count++
				}
			}
			side = append(side, e)
		}
	}
	add := func(title string, kind Kind, set map[string]int) {
		if len(set) == 0 {
			return
		}
		names := make([]string, 0, len(set))
		for n := range set {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
		side = append(side, Entry{Kind: Header, Value: title})
		for _, n := range names {
			side = append(side, Entry{Kind: kind, Value: n, Count: set[n]})
		}
	}
	add("PROJECTS", Project, projects)
	add("CONTEXTS", Context, contexts)

	for w := range used {
		known = append(known, w)
	}
	sort.Slice(known, func(i, j int) bool {
		if used[known[i]] != used[known[j]] {
			return used[known[i]] > used[known[j]]
		}
		return known[i] < known[j]
	})
	return side, known
}

// Find returns the index of the entry with that kind and value, or 0 (All).
func Find(side []Entry, kind Kind, value string) int {
	for i, e := range side {
		if e.Kind == kind && e.Value == value {
			return i
		}
	}
	return 0
}

// Visible returns the tasks shown for a sidebar entry and search, sorted.
func Visible(tasks []*todo.Task, e Entry, q query.Query, hideDone bool, mode string, now time.Time) []*todo.Task {
	var out []*todo.Task
	for _, t := range tasks {
		if (hideDone && t.Done) || !e.Match(t, now) || !q.Match(t, now) {
			continue
		}
		out = append(out, t)
	}
	Sort(mode, out, tasks, now)
	return out
}

// CurrentQuery is what "save search" saves: the search, plus the sidebar
// entry's project, context or saved query.
func CurrentQuery(e Entry, filter string, filters map[string]string) string {
	q := strings.TrimSpace(filter)
	switch e.Kind {
	case Project, Context:
		q = strings.TrimSpace(e.Value + " " + q)
	case Due:
		q = strings.TrimSpace("-due:none " + q)
	case Linked:
		q = strings.TrimSpace("kn: " + q)
	case Saved:
		q = strings.TrimSpace(filters[e.Value] + " " + q)
	}
	return q
}

// Sort orders visible under mode; tasks that tie keep their order in all
// (the file).
func Sort(mode string, visible, all []*todo.Task, now time.Time) {
	order := make(map[*todo.Task]int, len(all))
	for i, t := range all {
		order[t] = i
	}
	sort.SliceStable(visible, func(i, j int) bool {
		a, b := visible[i], visible[j]
		if c := Compare(mode, a, b, now); c != 0 {
			return c < 0
		}
		return order[a] < order[b]
	})
}

// MoveScope names what tasks must share to swap places under mode, for the
// "can only move within…" message.
func MoveScope(mode string) string {
	return map[string]string{
		"priority": "priority and due date", "due": "due date and priority", "created": "creation date",
	}[mode]
}

// Compare orders two tasks under a sort mode, ignoring file order; 0 means
// they tie and file order decides. Open tasks come before done ones. Under
// "priority", overdue tasks come first whatever their priority.
func Compare(mode string, a, b *todo.Task, now time.Time) int {
	if mode == "file" {
		return 0
	}
	if a.Done != b.Done {
		if !a.Done {
			return -1
		}
		return 1
	}
	pri := func(t *todo.Task) int {
		if t.Priority == 0 {
			return 27
		}
		return int(t.Priority - 'A')
	}
	cmpInt := func(x, y int) int {
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	cmpDue := func() int {
		da, oka := a.Due()
		db, okb := b.Due()
		switch {
		case oka != okb:
			if oka {
				return -1
			}
			return 1
		case oka && da.Before(db):
			return -1
		case oka && db.Before(da):
			return 1
		}
		return 0
	}
	switch mode {
	case "priority":
		today := Today(now)
		overdue := func(t *todo.Task) bool {
			due, ok := t.Due()
			return ok && due.Before(today)
		}
		if oa, ob := overdue(a), overdue(b); oa != ob {
			if oa {
				return -1
			}
			return 1
		}
		if c := cmpInt(pri(a), pri(b)); c != 0 {
			return c
		}
		return cmpDue()
	case "due":
		if c := cmpDue(); c != 0 {
			return c
		}
		return cmpInt(pri(a), pri(b))
	case "created":
		switch {
		case a.Created.Equal(b.Created):
			return 0
		case a.Created.IsZero():
			return 1
		case b.Created.IsZero():
			return -1
		case a.Created.After(b.Created):
			return -1
		}
		return 1
	}
	return 0
}

// Today is midnight at the start of now's day.
func Today(now time.Time) time.Time { return todo.Midnight(now) }

// DueKind says how a due date is colored.
type DueKind string

const (
	DueOverdue DueKind = "overdue" // past
	DueToday   DueKind = "today"
	DueSoon    DueKind = "due" // within a week
	DueLater   DueKind = "dim" // further off
)

// DueLabel is the badge for a due date: "overdue 3d", "today", "tomorrow",
// a weekday within a week, then "Jan 2" or a full date.
func DueLabel(d, now time.Time) (string, DueKind) {
	today := Today(now)
	days := int(math.Round(d.Sub(today).Hours() / 24))
	switch {
	case days < 0:
		return fmt.Sprintf("overdue %dd", -days), DueOverdue
	case days == 0:
		return "today", DueToday
	case days == 1:
		return "tomorrow", DueSoon
	case days < 7:
		return d.Format("Mon"), DueSoon
	case d.Year() == today.Year():
		return d.Format("Jan 2"), DueLater
	default:
		return d.Format("2006-01-02"), DueLater
	}
}

// LinkedNote returns the katnotes note (kind/id, from note:) a task is
// linked to by katnotes: it has a kn:<6 hex digits> tag. A task with kn:
// but no note: still counts, with an empty note.
func LinkedNote(t *todo.Task) (string, bool) {
	id, ok := t.Tag("kn")
	if !ok || !linkID(id) {
		return "", false
	}
	note, _ := t.Tag("note")
	return note, true
}

func linkID(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// IsLinkTag reports whether a word is a kn: or note: tag, drawn in the
// theme's link color.
func IsLinkTag(w string) bool {
	return (strings.HasPrefix(w, "kn:") || strings.HasPrefix(w, "note:")) && IsTag(w)
}

// IsTag reports whether a word is a key:value tag (not a URL, and not a
// time such as 14:30).
func IsTag(w string) bool {
	k, v, ok := strings.Cut(w, ":")
	if _, clock := todo.ParseClock(w, false); clock {
		return false
	}
	return ok && k != "" && v != "" && !strings.HasPrefix(v, "//")
}

// Body is a task's text without +projects, @contexts and key:value tags,
// as "copy text only" copies it.
func Body(t *todo.Task) string {
	var out []string
	for w := range strings.FieldsSeq(t.Text) {
		if (len(w) > 1 && (w[0] == '+' || w[0] == '@')) || IsTag(w) {
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// WorkLabels are the badges for an open task's status: and progress: tags:
// "▶ doing" (or the status as written, e.g. "blocked") and a progress bar
// such as "▰▰▱▱▱ 40%". They're empty when the tag is missing or the task is
// done; doing says whether the status is "doing", which is drawn brighter.
func WorkLabels(t *todo.Task) (status string, doing bool, progress string) {
	if t.Done {
		return "", false, ""
	}
	if s, ok := t.Status(); ok {
		status, doing = s, s == "doing"
		if doing {
			status = "▶ doing"
		}
	}
	if p, ok := t.Progress(); ok {
		filled := p / 20 // of 5 cells; only 100% fills them all
		progress = strings.Repeat("▰", filled) + strings.Repeat("▱", 5-filled) + fmt.Sprintf(" %d%%", p)
	}
	return status, doing, progress
}

// AlarmLabel is the badge for an open task's alarm: tag, e.g. "⏰ 7:30"
// today, "⏰ tomorrow 7:30", "⏰ Fri 7:30" this week, "⏰ Dec 1 7:30"; past
// is true once it has rung. Empty without one, or when the task is done.
func AlarmLabel(t *todo.Task, now time.Time) (label string, past bool) {
	a, ok := t.Alarm()
	if !ok || t.Done {
		return "", false
	}
	clock := todo.ClockOf(a)
	today := Today(now)
	days := int(math.Round(todo.Midnight(a).Sub(today).Hours() / 24))
	switch {
	case days == 0:
		label = clock
	case days == 1:
		label = "tomorrow " + clock
	case days > 1 && days < 7:
		label = a.Format("Mon ") + clock
	case a.Year() == now.Year():
		label = a.Format("Jan 2 ") + clock
	default:
		label = a.Format("2006-01-02 ") + clock
	}
	return "⏰ " + label, a.Before(now)
}

// Braced reports which words are part of a {{ … }} span, which may cover
// several words. A word counts if any of it is inside the braces, braces
// included. A {{ that is never closed marks nothing.
func Braced(words []string) []bool {
	out := make([]bool, len(words))
	open := -1
	for i, w := range words {
		for {
			if open < 0 {
				j := strings.Index(w, "{{")
				if j < 0 {
					break
				}
				open, w = i, w[j+2:]
				continue
			}
			j := strings.Index(w, "}}")
			if j < 0 {
				break
			}
			for k := open; k <= i; k++ {
				out[k] = true
			}
			open, w = -1, w[j+2:]
		}
	}
	return out
}
