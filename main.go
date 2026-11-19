// katTasks is a terminal UI for todo.txt files.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	fs := flag.CommandLine
	cfgPath := fs.String("config", config.DefaultPath(), "settings file")
	todoFile := fs.String("file", "", "todo.txt to open (overrides files.todo)")
	makeConfig := fs.Bool("makeconfig", false, "create the settings file with every default, then exit")
	updateConfig := fs.Bool("updateconfig", false, "add settings missing from the settings file (e.g. new in this version) as comments, then exit")
	makeTheme := fs.String("maketheme", "", "write the current theme to themes/`NAME`.toml next to the settings, to edit; then exit")
	showVersion := fs.Bool("version", false, "print the version, then exit")
	showPaths := fs.Bool("paths", false, "print the settings file, todo.txt and done.txt in use, then exit")
	asJSON := fs.Bool("json", false, "commands print JSON")
	force := fs.Bool("f", false, "commands don't ask before deleting")
	fs.Usage = func() { printUsage(fs, *cfgPath) }
	parseFlags(fs, os.Args[1:])
	if fs.Arg(0) == "help" { // katTasks help, like -h
		fs.SetOutput(os.Stdout)
		fs.Usage()
		return
	}

	switch {
	case *showVersion:
		fmt.Println("katTasks", Version())
		return
	case *showPaths:
		cfg, _, _ := config.Load(*cfgPath)
		if *todoFile != "" {
			cfg.SetTodo(*todoFile)
		}
		fmt.Printf("settings: %s\ntodo: %s\ndone: %s\n", *cfgPath, cfg.Files.Todo, cfg.Files.Done)
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

	if fs.Arg(0) == "remind" {
		remind(fs.Args()[1:], *cfgPath, *todoFile)
		return
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
	m, err := ui.New(cfg, ui.Options{ConfigPath: *cfgPath, TodoFile: *todoFile, Errors: errs, Version: Version()})
	exitIf(err)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go alert.Watch(ctx, alert.Options{Source: alert.FileSource(*cfgPath, *todoFile)})
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		exitIf(err)
	}
}

// remind is the katTasks remind command: the background reminder.
func remind(args []string, cfgPath, todoFile string) {
	rf := flag.NewFlagSet("remind", flag.ContinueOnError)
	test := rf.Bool("test", false, "send a sample alert now, with the [alerts] settings, then exit")
	testAlarm := rf.Bool("test-alarm", false, "ring a sample alarm now, with the [alarms] settings, then exit")
	install := rf.Bool("install", false, "start katTasks remind at login")
	uninstall := rf.Bool("uninstall", false, "stop starting it at login")
	rf.Usage = func() {
		fmt.Fprintf(rf.Output(), "Usage: katTasks [flags] remind [-test|-test-alarm|-install|-uninstall]\n\n"+
			"Without a flag it runs until stopped, sending the [alerts] and ringing the\n[alarms] of %s.\n\n", cfgPath)
		rf.PrintDefaults()
	}
	parseFlags(rf, args)

	switch {
	case *test:
		cfg, _, _ := config.Load(cfgPath)
		s := alert.SettingsOf(cfg)
		if !s.Popup && !s.Sound {
			exitIf(errors.New("alerts.popup and alerts.sound are both off"))
		}
		exitIf(alert.Test(s))
		fmt.Println("sent a test alert")
	case *testAlarm:
		cfg, _, _ := config.Load(cfgPath)
		s, err := alert.AlarmSettingsOf(cfg)
		exitIf(err)
		fmt.Printf("ringing for %s…\n", cfg.Alarms.Ring)
		exitIf(alert.TestAlarm(s))
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
		exitIf(err)
		fmt.Println(msg)
	case *uninstall:
		msg, err := alert.Uninstall()
		exitIf(err)
		fmt.Println(msg)
	default:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		alert.Watch(ctx, alert.Options{
			Source: alert.FileSource(cfgPath, todoFile),
			Logf:   func(f string, a ...any) { fmt.Fprintf(os.Stderr, "katTasks: "+f+"\n", a...) },
		})
	}
}

// parseFlags parses args with fs. Help asked for with -h or -help goes to
// stdout, so it can be piped to less, and exits 0; a mistake in the flags
// is reported on stderr with the help after it, and exits 2.
func parseFlags(fs *flag.FlagSet, args []string) {
	usage := fs.Usage
	fs.Usage = func() {} // printed below, to the right place
	fs.Init(fs.Name(), flag.ContinueOnError)
	err := fs.Parse(args)
	fs.Usage = usage
	switch {
	case errors.Is(err, flag.ErrHelp):
		fs.SetOutput(os.Stdout)
		usage()
		os.Exit(0)
	case err != nil: // flag has printed what was wrong
		usage()
		os.Exit(2)
	}
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "katTasks:", err)
		os.Exit(1)
	}
}
