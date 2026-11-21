// Package cli implements katTasks' one-shot commands, in the style of
// todo.sh: katTasks add "Call Mom", katTasks ls +Work, katTasks do 3.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/todo"
)

// Env is what a command needs.
type Env struct {
	Cfg   config.Config
	JSON  bool // machine-readable output
	Force bool // don't ask before deleting
	In    io.Reader
	Out   io.Writer
	Now   func() time.Time
}

type command struct {
	names []string
	usage string
	run   func(c *ctx, args []string) error
}

var commands = []command{
	{[]string{"add", "a"}, "add TEXT...", cmdAdd},
	{[]string{"append", "app"}, "append N TEXT...", cmdAppend},
	{[]string{"prepend", "prep"}, "prepend N TEXT...", cmdPrepend},
	{[]string{"replace"}, "replace N TEXT...", cmdReplace},
	{[]string{"pri", "p"}, "pri N PRIORITY", cmdPri},
	{[]string{"depri", "dp"}, "depri N...", cmdDepri},
	{[]string{"do", "done"}, "do N...", cmdDo},
	{[]string{"del", "rm"}, "del N [TERM]", cmdDel},
	{[]string{"start"}, "start N...", cmdStart},
	{[]string{"stop"}, "stop N...", cmdStop},
	{[]string{"status"}, "status N WORD|none", cmdStatus},
	{[]string{"progress", "prog"}, "progress N PERCENT|none", cmdProgress},
	{[]string{"at", "alert"}, "at N WHEN|none", cmdAt},
	{[]string{"alarm"}, "alarm N WHEN|none", cmdAlarm},
	{[]string{"alerts", "upcoming"}, "alerts [DAYS]", cmdAlerts},
	{[]string{"archive"}, "archive", cmdArchive},
	{[]string{"list", "ls"}, "ls [QUERY...]", cmdList},
	{[]string{"listall", "lsa"}, "lsa [QUERY...]", cmdListAll},
	{[]string{"listpri", "lsp"}, "lsp [PRIORITY or A-C]", cmdListPri},
	{[]string{"listproj", "lsprj"}, "lsprj", cmdListProj},
	{[]string{"listcon", "lsc"}, "lsc", cmdListCon},
}

func find(name string) *command {
	for i := range commands {
		for _, n := range commands[i].names {
			if n == name {
				return &commands[i]
			}
		}
	}
	return nil
}

// IsCommand reports whether name is a subcommand (or alias).
func IsCommand(name string) bool { return find(name) != nil }

// Help is one command's usage line, and the other names it answers to.
type Help struct {
	Usage   string // "add TEXT..."
	Aliases []string
}

// Commands lists the subcommands for katTasks -h.
func Commands() []Help {
	var out []Help
	for _, c := range commands {
		shown, _, _ := strings.Cut(c.usage, " ")
		h := Help{Usage: c.usage}
		for _, n := range c.names {
			if n != shown {
				h.Aliases = append(h.Aliases, n)
			}
		}
		out = append(out, h)
	}
	return out
}

// ErrUsage is returned for bad arguments; the message says what is wrong.
var ErrUsage = errors.New("usage")

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }
func (e usageErr) Unwrap() error { return ErrUsage }

// Run runs a subcommand. args[0] is its name.
func Run(env Env, args []string) error {
	if env.Now == nil {
		env.Now = time.Now
	}
	c := find(args[0])
	if c == nil {
		return usageErr{fmt.Sprintf("unknown command %q", args[0])}
	}
	x := &ctx{Env: env, name: c.names[0], usage: c.usage}
	if err := x.load(); err != nil {
		return err
	}
	return c.run(x, args[1:])
}

// ctx holds the loaded file for one command.
type ctx struct {
	Env
	name, usage string
	tasks       []*todo.Task
}

func (c *ctx) load() error {
	ts, _, err := todo.Load(c.Cfg.Files.Todo)
	c.tasks = ts
	return err
}

func (c *ctx) save() error {
	_, err := todo.Save(c.Cfg.Files.Todo, c.tasks)
	return err
}

func (c *ctx) bad(format string, a ...any) error {
	return usageErr{fmt.Sprintf(format, a...) + "\nusage: katTasks " + c.usage}
}

// num returns task N (1-based).
func (c *ctx) num(s string) (*todo.Task, int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, 0, c.bad("%q is not a task number", s)
	}
	if n < 1 || n > len(c.tasks) {
		return nil, 0, fmt.Errorf("no task %d (there are %d)", n, len(c.tasks))
	}
	return c.tasks[n-1], n, nil
}

func (c *ctx) index(t *todo.Task) int {
	for i, x := range c.tasks {
		if x == t {
			return i + 1
		}
	}
	return 0
}
