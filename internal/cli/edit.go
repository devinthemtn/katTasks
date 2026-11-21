// Commands that change tasks.

package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/devinthemtn/katTasks/internal/todo"
)

func cmdAdd(c *ctx, args []string) error {
	text := strings.TrimSpace(strings.Join(args, " "))
	if text == "" {
		return c.bad("nothing to add")
	}
	t := todo.NewTask(text, c.Now(), c.Cfg.Tasks.AddCreationDate)
	c.tasks = append(c.tasks, t)
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d added.", len(c.tasks)), []numbered{{len(c.tasks), t}}, nil)
}

func textArg(c *ctx, args []string) (*todo.Task, int, string, error) {
	if len(args) < 2 {
		return nil, 0, "", c.bad("need a task number and text")
	}
	t, n, err := c.num(args[0])
	return t, n, strings.TrimSpace(strings.Join(args[1:], " ")), err
}

func cmdAppend(c *ctx, args []string) error {
	t, n, text, err := textArg(c, args)
	if err != nil {
		return err
	}
	t.Text = strings.TrimSpace(t.Text + " " + text)
	t.Normalize(c.Now())
	if err := c.save(); err != nil {
		return err
	}
	return c.changed("", []numbered{{n, t}}, nil)
}

func cmdPrepend(c *ctx, args []string) error {
	t, n, text, err := textArg(c, args)
	if err != nil {
		return err
	}
	t.Text = strings.TrimSpace(text + " " + t.Text)
	t.Normalize(c.Now())
	if err := c.save(); err != nil {
		return err
	}
	return c.changed("", []numbered{{n, t}}, nil)
}

// cmdReplace swaps a task's text. A priority or creation date left out of
// the new text is kept from the old task.
func cmdReplace(c *ctx, args []string) error {
	t, n, text, err := textArg(c, args)
	if err != nil {
		return err
	}
	nt := todo.NewTask(text, c.Now(), false)
	if nt.Priority == 0 && !nt.Done {
		nt.Priority = t.Priority
	}
	if nt.Created.IsZero() {
		nt.Created = t.Created
	}
	old := t.String()
	*t = *nt
	if err := c.save(); err != nil {
		return err
	}
	if !c.JSON {
		fmt.Fprintf(c.Out, "%d %s\n", n, old)
	}
	return c.changed(fmt.Sprintf("Replaced task %d.", n), []numbered{{n, t}}, nil)
}

func cmdPri(c *ctx, args []string) error {
	if len(args) != 2 {
		return c.bad("need a task number and a priority")
	}
	t, n, err := c.num(args[0])
	if err != nil {
		return err
	}
	p := strings.ToUpper(args[1])
	if len(p) != 1 || p[0] < 'A' || p[0] > 'Z' {
		return c.bad("priority must be a letter A-Z")
	}
	if t.Done {
		return fmt.Errorf("task %d is done; completed tasks can't have a priority", n)
	}
	t.Priority = p[0]
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d prioritized (%s).", n, p), []numbered{{n, t}}, nil)
}

// several resolves task numbers to tasks before anything changes, so the
// numbers refer to the file as it was.
func several(c *ctx, args []string) ([]numbered, error) {
	if len(args) == 0 {
		return nil, c.bad("need at least one task number")
	}
	var out []numbered
	seen := map[int]bool{}
	for _, a := range args {
		for _, f := range strings.Split(a, ",") {
			if f == "" {
				continue
			}
			t, n, err := c.num(f)
			if err != nil {
				return nil, err
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, numbered{n, t})
			}
		}
	}
	return out, nil
}

func cmdDepri(c *ctx, args []string) error {
	ts, err := several(c, args)
	if err != nil {
		return err
	}
	for _, x := range ts {
		x.t.Priority = 0
	}
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d deprioritized.", len(ts)), ts, nil)
}

func cmdDo(c *ctx, args []string) error {
	ts, err := several(c, args)
	if err != nil {
		return err
	}
	now := c.Now()
	var nexts []*todo.Task
	var msgs []string
	for _, x := range ts {
		if x.t.Done {
			msgs = append(msgs, fmt.Sprintf("%d is already marked done.", x.n))
			continue
		}
		next, recurs := x.t.Next(now)
		x.t.Complete(now)
		if recurs {
			// After the original, as the TUI does.
			i := c.index(x.t)
			c.tasks = append(c.tasks[:i], append([]*todo.Task{next}, c.tasks[i:]...)...)
			nexts = append(nexts, next)
		}
		msgs = append(msgs, fmt.Sprintf("%d marked as done.", x.n))
	}
	if err := c.save(); err != nil {
		return err
	}
	// Report numbers as they are now that next copies were inserted.
	for i := range ts {
		ts[i].n = c.index(ts[i].t)
	}
	var added []numbered
	for _, n := range nexts {
		added = append(added, numbered{c.index(n), n})
	}
	if len(added) > 0 {
		msgs = append(msgs, fmt.Sprintf("%d recurring task(s) added.", len(added)))
	}
	return c.changed(strings.Join(msgs, " "), ts, added)
}

func cmdDel(c *ctx, args []string) error {
	if len(args) < 1 {
		return c.bad("need a task number")
	}
	t, n, err := c.num(args[0])
	if err != nil {
		return err
	}
	term := strings.TrimSpace(strings.Join(args[1:], " "))
	if term != "" {
		// Remove just TERM from the task.
		before := t.Text
		words := strings.Fields(t.Text)
		var keep []string
		for _, w := range words {
			if w != term {
				keep = append(keep, w)
			}
		}
		t.Text = strings.Join(keep, " ")
		if t.Text == before {
			t.Text = strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(" "+before+" ", " "+term+" ", " ")), " "))
		}
		if t.Text == before {
			return fmt.Errorf("%q not found in task %d", term, n)
		}
		if err := c.save(); err != nil {
			return err
		}
		return c.changed(fmt.Sprintf("Removed '%s' from task.", term), []numbered{{n, t}}, nil)
	}
	if !c.Force {
		if c.JSON {
			return c.bad("deleting with -json needs -f")
		}
		fmt.Fprintf(c.Out, "Delete '%s'?  (y/n) ", t)
		ans, _ := bufio.NewReader(c.In).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
			fmt.Fprintln(c.Out, "TODO: No tasks were deleted.")
			return nil
		}
	}
	c.tasks = append(c.tasks[:n-1], c.tasks[n:]...)
	if err := c.save(); err != nil {
		return err
	}
	return c.changed(fmt.Sprintf("%d deleted.", n), []numbered{{n, t}}, nil)
}

func cmdArchive(c *ctx, args []string) error {
	if len(args) > 0 {
		return c.bad("archive takes no arguments")
	}
	var done, keep []*todo.Task
	var report []numbered
	for i, t := range c.tasks {
		if t.Done {
			done = append(done, t)
			report = append(report, numbered{i + 1, t})
		} else {
			keep = append(keep, t)
		}
	}
	if len(done) > 0 {
		if _, err := todo.Archive(c.Cfg.Files.Todo, keep, c.Cfg.Files.Done, done); err != nil {
			return err
		}
		c.tasks = keep
	}
	return c.changed(fmt.Sprintf("%d task(s) archived to %s.", len(done), c.Cfg.Files.Done), report, nil)
}
