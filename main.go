// katTasks is a terminal UI for todo.txt files.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"katTasks/internal/config"
	"katTasks/internal/ui"
)

func main() {
	cfgPath := flag.String("config", config.DefaultPath(), "path to config.toml")
	flag.Parse()

	cfg, created, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "katTasks: config %s: %v\n", *cfgPath, err)
		os.Exit(1)
	}
	if created {
		fmt.Fprintf(os.Stderr, "katTasks: wrote default config to %s\n", *cfgPath)
	}

	m, err := ui.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "katTasks: %s: %v\n", cfg.TodoFile, err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "katTasks: %v\n", err)
		os.Exit(1)
	}
}
