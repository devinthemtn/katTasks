// Package todo parses and serializes tasks in the todo.txt format
// (https://github.com/todotxt/todo.txt).
package todo

import (
	"strings"
	"time"
)

// DateLayout is the date format used throughout todo.txt.
const DateLayout = "2006-01-02"

// Task is a single line of a todo.txt file.
type Task struct {
	Done      bool
	Priority  byte // 'A'..'Z', or 0 for none
	Completed time.Time
	Created   time.Time
	Text      string // description, including +projects, @contexts and key:value tags

	// The line as read, and its String() then, so a task that isn't
	// changed is written back exactly as it was.
	orig, origForm string
}

// bom is the byte order mark some Windows editors put at a file's start.
const bom = "\ufeff"

// Parse parses one todo.txt line. It never fails: anything it does not
// recognise is kept verbatim in Text.
func Parse(line string) *Task {
	t := &Task{}
	rest := strings.TrimPrefix(strings.TrimRight(line, "\r\n"), bom)
	t.orig = rest
	defer func() { t.origForm = t.String() }()

	if strings.HasPrefix(rest, "x ") {
		t.Done = true
		rest = rest[2:]
	}
	if p, r, ok := cutPriority(rest); ok {
		t.Priority, rest = p, r
	}
	if t.Done {
		if d, r, ok := cutDate(rest); ok {
			t.Completed, rest = d, r
			if d2, r2, ok := cutDate(rest); ok {
				t.Created, rest = d2, r2
			}
		}
	} else if d, r, ok := cutDate(rest); ok {
		t.Created, rest = d, r
	}
	t.Text = rest
	return t
}

func cutPriority(s string) (byte, string, bool) {
	if len(s) >= 4 && s[0] == '(' && s[1] >= 'A' && s[1] <= 'Z' && s[2] == ')' && s[3] == ' ' {
		return s[1], s[4:], true
	}
	return 0, s, false
}

func cutDate(s string) (time.Time, string, bool) {
	if len(s) < 10 || (len(s) > 10 && s[10] != ' ') {
		return time.Time{}, s, false
	}
	d, err := time.ParseInLocation(DateLayout, s[:10], time.Local)
	if err != nil {
		return time.Time{}, s, false
	}
	if len(s) == 10 {
		return d, "", true
	}
	return d, s[11:], true
}

// String serializes the task back to a todo.txt line.
func (t *Task) String() string {
	var b strings.Builder
	if t.Done {
		b.WriteString("x ")
	}
	if t.Priority != 0 {
		b.WriteByte('(')
		b.WriteByte(t.Priority)
		b.WriteString(") ")
	}
	if t.Done && !t.Completed.IsZero() {
		b.WriteString(t.Completed.Format(DateLayout))
		b.WriteByte(' ')
	}
	if !t.Created.IsZero() {
		b.WriteString(t.Created.Format(DateLayout))
		b.WriteByte(' ')
	}
	b.WriteString(t.Text)
	return strings.TrimRight(b.String(), " \t")
}

// line is the task as it goes in the file: the line it was read from if
// it hasn't changed since, so other programs' spacing is left alone.
func (t *Task) line() string {
	if t.orig != "" && t.String() == t.origForm {
		return t.orig
	}
	return t.String()
}

// Clone returns a copy of the task.
func (t *Task) Clone() *Task {
	c := *t
	return &c
}

// Complete marks the task done. Following todo.sh convention the priority is
// removed, but it is kept as a pri:X tag so Uncomplete can restore it.
func (t *Task) Complete(now time.Time) {
	if t.Done {
		return
	}
	t.Done = true
	t.Completed = Midnight(now)
	if t.Priority != 0 {
		t.SetTag("pri", string(t.Priority))
		t.Priority = 0
	}
}

// Uncomplete reverts Complete.
func (t *Task) Uncomplete() {
	if !t.Done {
		return
	}
	t.Done = false
	t.Completed = time.Time{}
	if p, ok := t.Tag("pri"); ok && len(p) == 1 && p[0] >= 'A' && p[0] <= 'Z' {
		t.Priority = p[0]
		t.RemoveTag("pri")
	}
}

// Projects returns the +project words in the description.
func (t *Task) Projects() []string { return t.prefixed('+') }

// Contexts returns the @context words in the description.
func (t *Task) Contexts() []string { return t.prefixed('@') }

func (t *Task) prefixed(p byte) []string {
	var out []string
	for _, w := range strings.Fields(t.Text) {
		if len(w) > 1 && w[0] == p {
			out = append(out, w)
		}
	}
	return out
}

// Has reports whether the description contains the exact word w.
func (t *Task) Has(w string) bool {
	for _, f := range strings.Fields(t.Text) {
		if f == w {
			return true
		}
	}
	return false
}

func splitTag(w string) (string, string, bool) {
	k, v, ok := strings.Cut(w, ":")
	if !ok || k == "" || v == "" || strings.ContainsAny(k, "+@") || strings.HasPrefix(v, "//") {
		return "", "", false
	}
	return k, v, true
}

// Tag returns the value of the first key:value tag with the given key.
func (t *Task) Tag(key string) (string, bool) {
	for _, w := range strings.Fields(t.Text) {
		if k, v, ok := splitTag(w); ok && k == key {
			return v, true
		}
	}
	return "", false
}

// SetTag sets key:value, replacing an existing tag or appending a new one.
func (t *Task) SetTag(key, value string) {
	words := strings.Fields(t.Text)
	for i, w := range words {
		if k, _, ok := splitTag(w); ok && k == key {
			words[i] = key + ":" + value
			t.Text = strings.Join(words, " ")
			return
		}
	}
	t.Text = strings.TrimSpace(t.Text + " " + key + ":" + value)
}

// RemoveTag removes every key:value tag with the given key.
func (t *Task) RemoveTag(key string) {
	words := strings.Fields(t.Text)
	out := words[:0]
	for _, w := range words {
		if k, _, ok := splitTag(w); ok && k == key {
			continue
		}
		out = append(out, w)
	}
	t.Text = strings.Join(out, " ")
}

// Due returns the parsed due:YYYY-MM-DD date, if any.
func (t *Task) Due() (time.Time, bool) {
	v, ok := t.Tag("due")
	if !ok {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation(DateLayout, v, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// Midnight is the start of t's day, in local time.
func Midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// Normalize keeps a task typed or edited by the user valid todo.txt:
// relative due dates (today, tomorrow, mon..sun, +Nd, +Nw, +Nm) become
// absolute, and the times in at: and alarm: tags lose their colons
// (at:14:30 becomes at:1430), since the format doesn't allow a colon in a
// tag's value. Tasks that aren't added or edited are never rewritten.
func (t *Task) Normalize(now time.Time) {
	if v, ok := t.Tag("due"); ok {
		if d, ok := ResolveDate(v, now); ok {
			t.SetTag("due", d.Format(DateLayout))
		}
	}
	if v, ok := t.Tag("at"); ok && strings.Contains(v, ":") {
		if min, ok := ParseClock(v, true); ok {
			t.SetTag("at", clockTag(v, min))
		}
	}
	if v, ok := t.Tag("alarm"); ok && strings.Contains(v, ":") {
		if a, ok := ParseAlarm(v); ok {
			t.SetTag("alarm", FormatAlarm(a))
		}
	}
}

// ResolveDate turns a relative date word into a date. Absolute dates are
// returned unchanged.
func ResolveDate(v string, now time.Time) (time.Time, bool) {
	today := Midnight(now)
	v = strings.ToLower(v)
	switch v {
	case "today", "tod":
		return today, true
	case "tomorrow", "tom":
		return today.AddDate(0, 0, 1), true
	}
	days := map[string]time.Weekday{
		"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
		"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
	}
	if len(v) >= 3 {
		if wd, ok := days[v[:3]]; ok {
			diff := (int(wd) - int(today.Weekday()) + 7) % 7
			if diff == 0 {
				diff = 7
			}
			return today.AddDate(0, 0, diff), true
		}
	}
	if len(v) >= 3 && v[0] == '+' {
		n := 0
		for _, c := range v[1 : len(v)-1] {
			if c < '0' || c > '9' {
				return time.Time{}, false
			}
			n = n*10 + int(c-'0')
		}
		switch v[len(v)-1] {
		case 'd':
			return today.AddDate(0, 0, n), true
		case 'w':
			return today.AddDate(0, 0, 7*n), true
		case 'm':
			return today.AddDate(0, n, 0), true
		}
	}
	return time.Time{}, false
}

// NewTask parses text typed by the user into a task: relative due dates
// become absolute, and today's creation date is added when addCreated is
// set and the text has none.
func NewTask(text string, now time.Time, addCreated bool) *Task {
	t := Parse(text)
	t.Normalize(now)
	if addCreated && t.Created.IsZero() && !t.Done {
		t.Created = Midnight(now)
	}
	return t
}
