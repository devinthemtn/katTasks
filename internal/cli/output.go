// Printing results, as text or JSON.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// Task is the JSON form of a task.
type Task struct {
	N         int               `json:"n"` // 0 for tasks in the done file
	Raw       string            `json:"raw"`
	Done      bool              `json:"done"`
	Priority  string            `json:"priority,omitempty"`
	Created   string            `json:"created,omitempty"`
	Completed string            `json:"completed,omitempty"`
	Text      string            `json:"text"`
	Projects  []string          `json:"projects"`
	Contexts  []string          `json:"contexts"`
	Tags      map[string]string `json:"tags"`
	Due       string            `json:"due,omitempty"`
	At        string            `json:"at,omitempty"`       // time of day, "14:30"
	Alarm     string            `json:"alarm,omitempty"`    // RFC 3339
	Status    string            `json:"status,omitempty"`   // "doing", …
	Progress  *int              `json:"progress,omitempty"` // percent
}

func toJSON(n int, t *todo.Task) Task {
	j := Task{N: n, Raw: t.String(), Done: t.Done, Text: t.Text,
		Projects: nonNil(t.Projects()), Contexts: nonNil(t.Contexts()), Tags: map[string]string{}}
	if t.Priority != 0 {
		j.Priority = string(t.Priority)
	}
	if !t.Created.IsZero() {
		j.Created = t.Created.Format(todo.DateLayout)
	}
	if !t.Completed.IsZero() {
		j.Completed = t.Completed.Format(todo.DateLayout)
	}
	for _, w := range strings.Fields(t.Text) {
		if k, v, ok := strings.Cut(w, ":"); ok && k != "" && v != "" && !strings.HasPrefix(v, "//") && !strings.ContainsAny(k, "+@") {
			if _, dup := j.Tags[k]; !dup {
				j.Tags[k] = v
			}
		}
	}
	if d, ok := t.Due(); ok {
		j.Due = d.Format(todo.DateLayout)
	}
	if v, ok := t.Tag("at"); ok {
		if min, ok := todo.ParseClock(v, true); ok {
			j.At = todo.FormatClock(min)
		}
	}
	if a, ok := t.Alarm(); ok {
		j.Alarm = a.Format(time.RFC3339)
	}
	j.Status, _ = t.Status()
	if p, ok := t.Progress(); ok {
		j.Progress = &p
	}
	return j
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type numbered struct {
	n int
	t *todo.Task
}

// Result is the JSON printed by commands that change tasks.
type Result struct {
	Action  string `json:"action"`
	Tasks   []Task `json:"tasks"`
	Added   []Task `json:"added,omitempty"` // e.g. the next copy of a recurring task
	Message string `json:"message,omitempty"`
}

func (c *ctx) printJSON(v any) error {
	enc := json.NewEncoder(c.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// changed reports tasks a command touched.
func (c *ctx) changed(msg string, ts []numbered, added []numbered) error {
	if c.JSON {
		r := Result{Action: c.name, Tasks: []Task{}, Message: msg}
		for _, x := range ts {
			r.Tasks = append(r.Tasks, toJSON(x.n, x.t))
		}
		for _, x := range added {
			r.Added = append(r.Added, toJSON(x.n, x.t))
		}
		return c.printJSON(r)
	}
	for _, x := range ts {
		fmt.Fprintf(c.Out, "%d %s\n", x.n, x.t)
	}
	for _, x := range added {
		fmt.Fprintf(c.Out, "%d %s\n", x.n, x.t)
	}
	if msg != "" {
		fmt.Fprintf(c.Out, "TODO: %s\n", msg)
	}
	return nil
}

// list prints tasks sorted by their full line, case-insensitively, as
// todo.sh does.
func (c *ctx) list(ts []numbered, total int) error {
	sort.SliceStable(ts, func(i, j int) bool {
		return strings.ToLower(ts[i].t.String()) < strings.ToLower(ts[j].t.String())
	})
	if c.JSON {
		out := []Task{}
		for _, x := range ts {
			out = append(out, toJSON(x.n, x.t))
		}
		return c.printJSON(out)
	}
	w := len(strconv.Itoa(max(total, 1)))
	for _, x := range ts {
		fmt.Fprintf(c.Out, "%0*d %s\n", w, x.n, x.t)
	}
	fmt.Fprintf(c.Out, "--\nTODO: %d of %d tasks shown\n", len(ts), total)
	return nil
}
