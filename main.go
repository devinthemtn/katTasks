// katTasks is a terminal UI for todo.txt files.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/devinthemtn/katTasks/internal/alert"
	"github.com/devinthemtn/katTasks/internal/cli"
	"github.com/devinthemtn/katTasks/internal/config"
	"github.com/devinthemtn/katTasks/internal/theme"
	"github.com/devinthemtn/katTasks/internal/ui"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is katTasks with the given arguments (without the program name) and
// streams, and returns its exit code: 0, 1 for an error, 2 for a mistake
// in how it was run.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintln(stderr, "katTasks:", err)
		return 1
	}

	fs := flag.NewFlagSet("katTasks", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", config.DefaultPath(), "settings file")
	todoFile := fs.String("file", "", "todo.txt to open (overrides files.todo)")
	makeConfig := fs.Bool("make-config", false, "create the settings file with every default, then exit")
	updateConfig := fs.Bool("update-config", false, "add settings missing from the settings file (e.g. new in this version) as comments, then exit")
	makeTheme := fs.String("make-theme", "", "write the current theme to themes/`NAME`.toml next to the settings, to edit; then exit")
	// The spellings before 1.0 still work, but help doesn't list them.
	fs.BoolVar(makeConfig, "makeconfig", false, "")
	fs.BoolVar(updateConfig, "updateconfig", false, "")
	fs.StringVar(makeTheme, "maketheme", "", "")
	showVersion := fs.Bool("version", false, "print the version, then exit")
	showPaths := fs.Bool("paths", false, "print the settings file, todo.txt and done.txt in use, then exit")
	asJSON := fs.Bool("json", false, "commands print JSON")
	force := fs.Bool("f", false, "commands don't ask before deleting")
	fs.Usage = func() { printUsage(fs, *cfgPath) }
	if code, ok := parseFlags(fs, args, stdout); !ok {
		return code
	}
	if fs.Arg(0) == "help" { // katTasks help, like -h
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}

	switch {
	case *showVersion:
		fmt.Fprintln(stdout, "katTasks", Version())
		return 0
	case *showPaths:
		cfg, _, _ := config.Load(*cfgPath)
		if *todoFile != "" {
			cfg.SetTodo(*todoFile)
		}
		fmt.Fprintf(stdout, "settings: %s\ntodo: %s\ndone: %s\n", *cfgPath, cfg.Files.Todo, cfg.Files.Done)
		return 0
	case *makeConfig:
		created, err := config.Make(*cfgPath)
		if err != nil {
			return fail(err)
		}
		if created {
			fmt.Fprintln(stdout, "created", *cfgPath)
		} else {
			fmt.Fprintln(stdout, *cfgPath, "already exists; left unchanged (try -update-config)")
		}
		return 0
	case *makeTheme != "":
		cfg, _, _ := config.Load(*cfgPath)
		themes, _ := theme.Load(cfg.Dir())
		cur, ok := theme.Find(themes, cfg.Theme)
		if !ok {
			cur, _ = theme.Find(themes, "default")
		}
		p, err := theme.Write(cfg.Dir(), *makeTheme, cur)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "wrote %s (a copy of %q); set theme = %q to use it\n", p, cur.Name, *makeTheme)
		return 0
	case *updateConfig:
		added, created, err := config.Update(*cfgPath)
		if err != nil {
			return fail(err)
		}
		switch {
		case created:
			fmt.Fprintln(stdout, "created", *cfgPath)
		case len(added) == 0:
			fmt.Fprintln(stdout, *cfgPath, "already has every setting")
		default:
			fmt.Fprintf(stdout, "added %d setting(s) to %s as comments:\n", len(added), *cfgPath)
			for _, a := range added {
				fmt.Fprintln(stdout, "  "+a)
			}
		}
		return 0
	}
	cfg, created, errs := config.Load(*cfgPath)
	if *todoFile != "" {
		cfg.SetTodo(*todoFile)
	}

	if fs.Arg(0) == "remind" {
		return remind(fs.Args()[1:], *cfgPath, *todoFile, stdout, stderr)
	}
	if fs.NArg() > 0 {
		if !cli.IsCommand(fs.Arg(0)) {
			fmt.Fprintf(stderr, "katTasks: unknown command %q\n\n", fs.Arg(0))
			fs.Usage()
			return 2
		}
		for _, e := range errs {
			fmt.Fprintln(stderr, "katTasks: warning:", e)
		}
		err := cli.Run(cli.Env{Cfg: cfg, JSON: *asJSON, Force: *force, In: stdin, Out: stdout}, fs.Args())
		if errors.Is(err, cli.ErrUsage) {
			fmt.Fprintln(stderr, "katTasks:", err)
			return 2
		}
		if err != nil {
			return fail(err)
		}
		return 0
	}

	if created {
		fmt.Fprintf(stderr, "katTasks: wrote default settings to %s\n", *cfgPath)
	}
	m, err := ui.New(cfg, ui.Options{ConfigPath: *cfgPath, TodoFile: *todoFile, Errors: errs, Version: Version()})
	if err != nil {
		return fail(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go alert.Watch(ctx, alert.Options{Source: alert.FileSource(*cfgPath, *todoFile)})
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		return fail(err)
	}
	return 0
}

// remind is the katTasks remind command: the background reminder.
func remind(args []string, cfgPath, todoFile string, stdout, stderr io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintln(stderr, "katTasks:", err)
		return 1
	}
	rf := flag.NewFlagSet("remind", flag.ContinueOnError)
	rf.SetOutput(stderr)
	test := rf.Bool("test", false, "send a sample alert now, with the [alerts] settings, then exit")
	testAlarm := rf.Bool("test-alarm", false, "ring a sample alarm now, with the [alarms] settings, then exit")
	install := rf.Bool("install", false, "start katTasks remind at login")
	uninstall := rf.Bool("uninstall", false, "stop starting it at login")
	rf.Usage = func() {
		fmt.Fprintf(rf.Output(), "Usage: katTasks [flags] remind [-test|-test-alarm|-install|-uninstall]\n\n"+
			"Without a flag it runs until stopped, sending the [alerts] and ringing the\n[alarms] of %s.\n\n", cfgPath)
		rf.PrintDefaults()
	}
	if code, ok := parseFlags(rf, args, stdout); !ok {
		return code
	}

	switch {
	case *test:
		cfg, _, _ := config.Load(cfgPath)
		s := alert.SettingsOf(cfg)
		if !s.Popup && !s.Sound {
			return fail(errors.New("alerts.popup and alerts.sound are both off"))
		}
		if err := alert.Test(s); err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, "sent a test alert")
	case *testAlarm:
		cfg, _, _ := config.Load(cfgPath)
		s, err := alert.AlarmSettingsOf(cfg)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "ringing for %s…\n", cfg.Alarms.Ring)
		if err := alert.TestAlarm(s); err != nil {
			return fail(err)
		}
	case *install:
		// The same settings and todo file as this run.
		var argv []string
		if cfgPath != config.DefaultPath() {
			abs, _ := filepath.Abs(config.Expand(cfgPath))
			argv = append(argv, "-config", abs)
		}
		if todoFile != "" {
			abs, _ := filepath.Abs(config.Expand(todoFile))
			argv = append(argv, "-file", abs)
		}
		msg, err := alert.Install(append(argv, "remind"))
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, msg)
	case *uninstall:
		msg, err := alert.Uninstall()
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, msg)
	default:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		alert.Watch(ctx, alert.Options{
			Source: alert.FileSource(cfgPath, todoFile),
			Logf:   func(f string, a ...any) { fmt.Fprintf(stderr, "katTasks: "+f+"\n", a...) },
		})
	}
	return 0
}

// parseFlags parses args with fs. Help asked for with -h or -help goes to
// stdout, so it can be piped to less; a mistake in the flags is reported
// on the flag set's output with the help after it. It returns false when
// katTasks should stop, with the exit code: 0 after help, 2 after a
// mistake.
func parseFlags(fs *flag.FlagSet, args []string, stdout io.Writer) (code int, ok bool) {
	usage := fs.Usage
	fs.Usage = func() {} // printed below, to the right place
	err := fs.Parse(args)
	fs.Usage = usage
	switch {
	case errors.Is(err, flag.ErrHelp):
		fs.SetOutput(stdout)
		usage()
		return 0, false
	case err != nil: // flag has printed what was wrong
		usage()
		return 2, false
	}
	return 0, true
}
