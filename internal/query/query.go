// Package query parses and matches katTasks search queries.
//
// A query is a list of terms separated by spaces; a task must match every
// term. Terms:
//
//	word        the line contains word (smartcase: case-insensitive unless
//	            word has an uppercase letter); "two words" in quotes is one term
//	+proj @ctx  the task has exactly that project / context (smartcase)
//	due:+1w     due between today and 1 week from now; due:-3d is the 3 days
//	            up to today; the sign defaults to +. Units d, b (business
//	            days), w, m, y
//	due:today   due:overdue, due:none, or an exact date like due:2026-12-01
//	pri:A       priority A; pri:A-C a range; pri:none no priority
//	status:doing  that status: tag; status:any has one, status:none hasn't
//	progress:50 that percent in progress:; progress:>50, progress:<50,
//	            progress:25-75 (inclusive), progress:any, progress:none
//	-term       any of the above, negated
package query

import (
	"strings"
	"time"
	"unicode"

	"github.com/devinthemtn/katTasks/internal/todo"
)

type kind int

const (
	kText kind = iota
	kWord      // +project / @context
	kDueRange
	kDueDate
	kDueDay // today / tomorrow, relative to the time Match is given
	kDueOverdue
	kDueNone
	kPri
	kStatus
	kProgress
)

type term struct {
	kind  kind
	neg   bool
	text  string // kText, kWord
	exact bool   // smartcase: text has an uppercase letter

	rec  todo.Recurrence // kDueRange
	back bool            // kDueRange: range runs into the past
	date time.Time       // kDueDate
	days int             // kDueDay: 0 today, 1 tomorrow

	priLo, priHi byte // kPri; 0, 0 means no priority

	status string // kStatus: lowercased; "" none, "*" any

	pctLo, pctHi int // kProgress, inclusive; -1, -1 means no progress tag
}

// Query is a parsed search.
type Query struct{ terms []term }

// Empty reports whether the query has no terms.
func (q Query) Empty() bool { return len(q.terms) == 0 }

// split breaks q into terms on spaces, keeping "quoted phrases" (and
// -"negated phrases") together.
func split(q string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	for _, r := range q {
		switch {
		case r == '"':
			inQuote = !inQuote
		case unicode.IsSpace(r) && !inQuote:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// Parse parses a query. It never fails: anything that isn't a recognised
// term is matched as text.
func Parse(q string) Query {
	var out Query
	for _, raw := range split(q) {
		t := term{}
		s := raw
		if len(s) > 1 && s[0] == '-' {
			t.neg, s = true, s[1:]
		}
		parseTerm(&t, s)
		out.terms = append(out.terms, t)
	}
	return out
}

func parseTerm(t *term, s string) {
	t.kind, t.text, t.exact = kText, s, hasUpper(s)
	switch {
	case len(s) > 1 && (s[0] == '+' || s[0] == '@'):
		t.kind = kWord
	case strings.HasPrefix(strings.ToLower(s), "due:") && len(s) > 4:
		parseDue(t, s[4:])
	case strings.HasPrefix(strings.ToLower(s), "pri:") && len(s) > 4:
		parsePri(t, s[4:])
	case strings.HasPrefix(strings.ToLower(s), "status:") && len(s) > 7:
		t.kind, t.status = kStatus, strings.ToLower(s[7:])
		switch t.status {
		case "none":
			t.status = ""
		case "any":
			t.status = "*"
		}
	case strings.HasPrefix(strings.ToLower(s), "progress:") && len(s) > 9:
		parseProgress(t, strings.ToLower(s[9:]))
	}
}

func parseDue(t *term, v string) {
	lv := strings.ToLower(v)
	switch lv {
	case "overdue":
		t.kind = kDueOverdue
		return
	case "none":
		t.kind = kDueNone
		return
	}
	if d, err := time.ParseInLocation(todo.DateLayout, v, time.Local); err == nil {
		t.kind, t.date = kDueDate, d
		return
	}
	switch lv {
	case "today", "tod":
		t.kind, t.days = kDueDay, 0
		return
	case "tomorrow", "tom":
		t.kind, t.days = kDueDay, 1
		return
	}
	back := false
	switch {
	case strings.HasPrefix(lv, "-"):
		back, lv = true, lv[1:]
	case strings.HasPrefix(lv, "+"):
		lv = lv[1:]
	}
	if r, ok := todo.ParseRecurrence(lv); ok && !r.Strict {
		t.kind, t.rec, t.back = kDueRange, r, back
	}
}

func parsePri(t *term, v string) {
	v = strings.ToUpper(v)
	if v == "NONE" {
		t.kind = kPri
		return
	}
	isPri := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	switch {
	case len(v) == 1 && isPri(v[0]):
		t.kind, t.priLo, t.priHi = kPri, v[0], v[0]
	case len(v) == 3 && isPri(v[0]) && v[1] == '-' && isPri(v[2]):
		lo, hi := v[0], v[2]
		if lo > hi {
			lo, hi = hi, lo
		}
		t.kind, t.priLo, t.priHi = kPri, lo, hi
	}
}

func parseProgress(t *term, v string) {
	lo, hi, ok := 0, 0, false
	switch {
	case v == "none":
		lo, hi, ok = -1, -1, true
	case v == "any":
		lo, hi, ok = 0, 100, true
	case strings.HasPrefix(v, ">"):
		lo, ok = todo.ParsePercent(v[1:])
		lo, hi = lo+1, 100
	case strings.HasPrefix(v, "<"):
		hi, ok = todo.ParsePercent(v[1:])
		lo, hi = 0, hi-1
	case strings.Contains(v, "-"):
		a, b, _ := strings.Cut(v, "-")
		var okA, okB bool
		lo, okA = todo.ParsePercent(a)
		hi, okB = todo.ParsePercent(b)
		ok = okA && okB
		if lo > hi {
			lo, hi = hi, lo
		}
	default:
		lo, ok = todo.ParsePercent(v)
		hi = lo
	}
	if ok {
		t.kind, t.pctLo, t.pctHi = kProgress, lo, hi
	}
}

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// back returns today moved back by the interval.
func back(today time.Time, r todo.Recurrence) time.Time {
	switch r.Unit {
	case 'd':
		return today.AddDate(0, 0, -r.N)
	case 'w':
		return today.AddDate(0, 0, -7*r.N)
	case 'm':
		return today.AddDate(0, -r.N, 0)
	case 'y':
		return today.AddDate(-r.N, 0, 0)
	case 'b':
		return todo.AddBusinessDays(today, -r.N)
	}
	return today
}

// Match reports whether task t matches every term, as of now.
func (q Query) Match(t *todo.Task, now time.Time) bool {
	if len(q.terms) == 0 {
		return true
	}
	line := t.String()
	lower := strings.ToLower(line)
	today := day(now)
	for _, tm := range q.terms {
		if tm.match(t, line, lower, today) == tm.neg {
			return false
		}
	}
	return true
}

func (tm term) match(t *todo.Task, line, lower string, today time.Time) bool {
	switch tm.kind {
	case kText:
		if tm.exact {
			return strings.Contains(line, tm.text)
		}
		return strings.Contains(lower, strings.ToLower(tm.text))
	case kWord:
		for _, w := range strings.Fields(t.Text) {
			if w == tm.text || (!tm.exact && strings.EqualFold(w, tm.text)) {
				return true
			}
		}
		return false
	case kDueNone:
		_, ok := t.Due()
		return !ok
	case kPri:
		if tm.priLo == 0 {
			return t.Priority == 0
		}
		return t.Priority >= tm.priLo && t.Priority <= tm.priHi
	case kStatus:
		s, ok := t.Status()
		switch tm.status {
		case "":
			return !ok
		case "*":
			return ok
		}
		return ok && s == tm.status
	case kProgress:
		p, ok := t.Progress()
		if tm.pctLo < 0 {
			return !ok
		}
		return ok && p >= tm.pctLo && p <= tm.pctHi
	}
	due, ok := t.Due()
	if !ok {
		return false
	}
	switch tm.kind {
	case kDueDate:
		return due.Equal(tm.date)
	case kDueDay:
		return due.Equal(today.AddDate(0, 0, tm.days))
	case kDueOverdue:
		return due.Before(today) && !t.Done
	case kDueRange:
		from, to := today, tm.rec.Add(today)
		if tm.back {
			from, to = back(today, tm.rec), today
		}
		return !due.Before(from) && !due.After(to)
	}
	return false
}

// Highlights reports whether word (one word of a task's text) should be
// marked as matching a plain-text term of the query.
func (q Query) Highlights(word string) bool {
	lw := strings.ToLower(word)
	for _, tm := range q.terms {
		if tm.neg || tm.kind == kDueRange || tm.kind == kDueDate || tm.kind == kDueDay || tm.kind == kDueOverdue || tm.kind == kDueNone || tm.kind == kPri || tm.kind == kStatus || tm.kind == kProgress {
			continue
		}
		switch {
		case tm.kind == kWord && (word == tm.text || (!tm.exact && strings.EqualFold(word, tm.text))):
			return true
		case tm.kind == kText && !strings.Contains(tm.text, " ") && tm.exact && strings.Contains(word, tm.text):
			return true
		case tm.kind == kText && !strings.Contains(tm.text, " ") && !tm.exact && strings.Contains(lw, strings.ToLower(tm.text)):
			return true
		case tm.kind == kText && strings.Contains(tm.text, " "):
			for _, part := range strings.Fields(tm.text) {
				if strings.Contains(lw, strings.ToLower(part)) {
					return true
				}
			}
		}
	}
	return false
}
