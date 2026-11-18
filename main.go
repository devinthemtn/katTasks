// katTasks is a terminal UI for todo.txt files.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/cli"
	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/theme"
	"github.com/devinthemtn/katTasks/internal/ui"
)

func main() {
	fs := flag.CommandLine
	cfgPath := fs.String("config", config.DefaultPath(), "settings file")
	todoFile := fs.String("file", "", "todo.txt to open (overrides files.todo)")
	makeConfig := fs.Bool("makeconfig", false, "create the settings file with every default, then exit")
	updateConfig := fs.Bool("updateconfig", false, "add settings missing from the settings file (e.g. new in this version) as comments, then exit")
	makeTheme := fs.String("maketheme", "", "write the current theme to themes/`NAME`.toml next to the settings, to edit; then exit")
	showVersion := fs.Bool("version", false, "print the version, then exit")
	asJSON := fs.Bool("json", false, "commands print JSON")
	force := fs.Bool("f", false, "commands don't ask before deleting")
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage: katTasks [flags]                 open the task list\n")
		fmt.Fprintf(out, "       katTasks [flags] COMMAND ARGS  run one command and exit\n\nFlags (before the command):\n")
		fs.PrintDefaults()
		fmt.Fprintf(out, "\nCommands (N is a task number, as L shows in the task list):\n%s", cli.Usage())
		fmt.Fprintf(out, "\nQueries for ls/lsa: words, +proj, @ctx, due:+1w, due:-3d, due:overdue, pri:A-C, -term\n")
		fmt.Fprintf(out, "\nSettings: %s\n", *cfgPath)
	}
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Println("katTasks", Version)
		return
	case *makeConfig:
		created, err := config.Make(*cfgPath)
		exitIf(err)
		if created {
			fmt.Println("created", *cfgPath)
		} else {
			fmt.Println(*cfgPath, "already exists; left unchanged (try -updateconfig)")
		}
		return
	case *makeTheme != "":
		cfg, _, _ := config.Load(*cfgPath)
		themes, _ := theme.Load(cfg.Dir())
		cur, ok := theme.Find(themes, cfg.Theme)
		if !ok {
			cur, _ = theme.Find(themes, "default")
		}
		p, err := theme.Write(cfg.Dir(), *makeTheme, cur)
		exitIf(err)
		fmt.Printf("wrote %s (a copy of %q); set theme = %q to use it\n", p, cur.Name, *makeTheme)
		return
	case *updateConfig:
		added, created, err := config.Update(*cfgPath)
		exitIf(err)
		switch {
		case created:
			fmt.Println("created", *cfgPath)
		case len(added) == 0:
			fmt.Println(*cfgPath, "already has every setting")
		default:
			fmt.Printf("added %d setting(s) to %s as comments:\n", len(added), *cfgPath)
			for _, a := range added {
				fmt.Println("  " + a)
			}
		}
		return
	}
	cfg, created, errs := config.Load(*cfgPath)
	if *todoFile != "" {
		cfg.SetTodo(*todoFile)
	}

	if fs.NArg() > 0 {
		if !cli.IsCommand(fs.Arg(0)) {
			fmt.Fprintf(os.Stderr, "katTasks: unknown command %q\n\n", fs.Arg(0))
			fs.Usage()
			os.Exit(2)
		}
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "katTasks: warning:", e)
		}
		err := cli.Run(cli.Env{Cfg: cfg, JSON: *asJSON, Force: *force, In: os.Stdin, Out: os.Stdout}, fs.Args())
		if errors.Is(err, cli.ErrUsage) {
			fmt.Fprintln(os.Stderr, "katTasks:", err)
			os.Exit(2)
		}
		exitIf(err)
		return
	}

	if created {
		fmt.Fprintf(os.Stderr, "katTasks: wrote default settings to %s\n", *cfgPath)
	}
	m, err := ui.New(cfg, ui.Options{ConfigPath: *cfgPath, TodoFile: *todoFile, Errors: errs})
	exitIf(err)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		exitIf(err)
	}
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "katTasks:", err)
		os.Exit(1)
	}
}
