package todo

import (
	"strconv"
	"strings"
)

// The todo.txt format only has done and not done. Work in between is kept in
// two ordinary key:value tags, which other todo.txt tools keep and ignore:
//
//	status:doing    being worked on (or any word: status:blocked, status:review)
//	progress:40     40% complete, 0–100

// Status returns the lowercased value of the status: tag, if any.
func (t *Task) Status() (string, bool) {
	v, ok := t.Tag("status")
	return strings.ToLower(v), ok
}

// Progress returns the percent in the progress: tag. A trailing % is
// allowed; a value that isn't a whole number from 0 to 100 doesn't count.
func (t *Task) Progress() (int, bool) {
	v, ok := t.Tag("progress")
	if !ok {
		return 0, false
	}
	return ParsePercent(v)
}

// ParsePercent parses "40" or "40%" as a percent from 0 to 100.
func ParsePercent(v string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSuffix(v, "%"))
	if err != nil || n < 0 || n > 100 {
		return 0, false
	}
	return n, true
}

// SetProgress sets progress:n, capped at 100. n <= 0 removes the tag.
func (t *Task) SetProgress(n int) {
	if n <= 0 {
		t.RemoveTag("progress")
		return
	}
	t.SetTag("progress", strconv.Itoa(min(n, 100)))
}

// IsWorkTag reports whether word is a status: or progress: tag that Status
// or Progress would read, so a row can show it as a badge instead.
func IsWorkTag(word string) bool {
	k, v, ok := splitTag(word)
	switch {
	case !ok:
		return false
	case k == "status":
		return true
	case k == "progress":
		_, ok := ParsePercent(v)
		return ok
	}
	return false
}
