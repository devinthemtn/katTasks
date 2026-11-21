// Commands that list tasks, projects and contexts.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/devinthemtn/katTasks/internal/query"
	"github.com/devinthemtn/katTasks/internal/todo"
)

func (c *ctx) matching(args []string) []numbered {
	q := query.Parse(strings.Join(args, " "))
	now := c.Now()
	var out []numbered
	for i, t := range c.tasks {
		if q.Match(t, now) {
			out = append(out, numbered{i + 1, t})
		}
	}
	return out
}

func cmdList(c *ctx, args []string) error {
	return c.list(c.matching(args), len(c.tasks))
}

func cmdListAll(c *ctx, args []string) error {
	ts := c.matching(args)
	done, _, err := todo.Load(c.Cfg.Files.Done)
	if err != nil {
		return err
	}
	q := query.Parse(strings.Join(args, " "))
	now := c.Now()
	for _, t := range done {
		if q.Match(t, now) {
			ts = append(ts, numbered{0, t})
		}
	}
	return c.list(ts, len(c.tasks)+len(done))
}

func cmdListPri(c *ctx, args []string) error {
	if len(args) > 1 {
		return c.bad("one priority or range at most")
	}
	q := "-pri:none"
	if len(args) == 1 {
		p := strings.ToUpper(args[0])
		letter := func(b byte) bool { return b >= 'A' && b <= 'Z' }
		if !(len(p) == 1 && letter(p[0])) && !(len(p) == 3 && letter(p[0]) && p[1] == '-' && letter(p[2])) {
			return c.bad("%q is not a priority (want a letter like A, or a range like A-C)", args[0])
		}
		q = "pri:" + p
	}
	ts := c.matching([]string{q})
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].t.Priority < ts[j].t.Priority })
	if c.JSON {
		out := []Task{}
		for _, x := range ts {
			out = append(out, toJSON(x.n, x.t))
		}
		return c.printJSON(out)
	}
	w := len(strconv.Itoa(max(len(c.tasks), 1)))
	for _, x := range ts {
		fmt.Fprintf(c.Out, "%0*d %s\n", w, x.n, x.t)
	}
	fmt.Fprintf(c.Out, "--\nTODO: %d of %d tasks shown\n", len(ts), len(c.tasks))
	return nil
}

func (c *ctx) words(get func(*todo.Task) []string) error {
	seen := map[string]bool{}
	var out []string
	for _, t := range c.tasks {
		for _, w := range get(t) {
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	if c.JSON {
		return c.printJSON(nonNil(out))
	}
	for _, w := range out {
		fmt.Fprintln(c.Out, w)
	}
	return nil
}

func cmdListProj(c *ctx, _ []string) error { return c.words((*todo.Task).Projects) }
func cmdListCon(c *ctx, _ []string) error  { return c.words((*todo.Task).Contexts) }
