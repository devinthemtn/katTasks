package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/devinthemtn/katTasks/internal/cli"
)

// examples end katTasks -h: a command and what it does.
var examples = [][2]string{
	{`katTasks add "Call mom +Family due:fri"`, "add a task, due on Friday"},
	{`katTasks add "(A) Pay rent due:2026-12-01 rec:+1m"`, "a top-priority task that repeats monthly"},
	{"katTasks ls", "list every open task"},
	{"katTasks ls +Work due:+1w -@waiting", "+Work due within a week, not @waiting"},
	{"katTasks do 3", "complete task 3 (L in the task list shows numbers)"},
	{"katTasks pri 3 A", "give task 3 priority A"},
	{"katTasks append 3 @phone", "add words to the end of task 3"},
	{"katTasks start 3", "mark task 3 as being worked on"},
	{"katTasks progress 3 50", "task 3 is half done"},
	{"katTasks alert +5m Check the oven", "a new task with a popup in 5 minutes"},
	{"katTasks alarm +5m Tea is ready", "a new task that rings like an alarm in 5 minutes"},
	{"katTasks alarm 3 tomorrow 6:30", "an alarm on task 3"},
	{"katTasks at 3 930", "task 3 at 9:30: the agenda, and an alert each day"},
	{"katTasks alerts", "the alerts and alarms coming up this week"},
	{"katTasks remind -install", "send alerts and alarms with katTasks closed"},
	{"katTasks -json ls pri:A | jq '.[].text'", "JSON for scripts"},
}

// helpStyles color katTasks -h. They use the terminal's own ANSI colors,
// so they follow its theme, and print plain text when w isn't a terminal
// (katTasks -h | less) or NO_COLOR is set.
type helpStyles struct{ head, cmd, flag, dim lipgloss.Style }

func newHelpStyles(w io.Writer) helpStyles {
	r := lipgloss.NewRenderer(w)
	return helpStyles{
		head: r.NewStyle().Bold(true).Foreground(lipgloss.Color("5")),
		cmd:  r.NewStyle().Foreground(lipgloss.Color("6")),
		flag: r.NewStyle().Foreground(lipgloss.Color("2")),
		dim:  r.NewStyle().Faint(true),
	}
}

// printUsage writes katTasks -h to fs's output.
func printUsage(fs *flag.FlagSet, cfgPath string) {
	w := fs.Output()
	st := newHelpStyles(w)
	head := func(s string) { fmt.Fprintln(w, st.head.Render(s)) }

	head("Usage")
	fmt.Fprintf(w, "  %s %s  %s\n", st.cmd.Render("katTasks"), fmt.Sprintf("%-22s", "[flags]"), st.dim.Render("open the task list"))
	fmt.Fprintf(w, "  %s %s  %s\n\n", st.cmd.Render("katTasks"), fmt.Sprintf("%-22s", "[flags] COMMAND ARGS"), st.dim.Render("run one command and exit"))

	head("Flags (before the command)")
	var flags bytes.Buffer
	fs.SetOutput(&flags)
	fs.PrintDefaults()
	fs.SetOutput(w)
	for _, line := range strings.Split(strings.TrimRight(flags.String(), "\n"), "\n") {
		if name, rest, ok := strings.Cut(strings.TrimPrefix(line, "  "), " "); ok && strings.HasPrefix(line, "  -") {
			fmt.Fprintln(w, "  "+st.flag.Render(name)+" "+rest)
		} else if strings.HasPrefix(line, "  -") {
			fmt.Fprintln(w, "  "+st.flag.Render(strings.TrimSpace(line)))
		} else {
			fmt.Fprintln(w, st.dim.Render(line))
		}
	}
	fmt.Fprintln(w)

	head("Commands" + st.dim.Render(" (N is a task number, as L shows in the task list)"))
	cmds := cli.Commands()
	width := 0
	for _, c := range cmds {
		width = max(width, len(c.Usage))
	}
	cmds = append(cmds, cli.Help{Usage: "remind [-test|-test-alarm|-install|-uninstall]"}) // too long to set the width
	for _, c := range cmds {
		name, args, _ := strings.Cut(c.Usage, " ")
		line := "  " + st.cmd.Render("katTasks "+name) + " " + fmt.Sprintf("%-*s", width-len(name), args)
		if len(c.Aliases) > 0 {
			line += "  " + st.dim.Render("("+strings.Join(c.Aliases, ", ")+")")
		}
		fmt.Fprintln(w, strings.TrimRight(line, " "))
	}
	fmt.Fprintln(w, st.dim.Render("      sends [alerts] and rings [alarms] in the background; see katTasks remind -h"))
	fmt.Fprintln(w)

	head("Queries for ls and lsa")
	fmt.Fprintln(w, "  words +proj @ctx due:+1w due:-3d due:overdue due:none pri:A-C pri:none")
	fmt.Fprintln(w, "  status:doing status:any progress:>50 progress:none, and -term for not")
	fmt.Fprintln(w)

	head("WHEN for at (alert) and alarm")
	fmt.Fprintln(w, "  930  14:30  3pm  +5m  in 2h  tomorrow 6:30  fri 7am  2026-12-01 0730")
	fmt.Fprintln(w, st.dim.Render("  With words instead of N they add a task: katTasks alarm +5m Tea is ready."))
	fmt.Fprintln(w, st.dim.Render("  PERCENT for progress: 40, +10, -10."))
	fmt.Fprintln(w)

	head("Examples")
	ew := 0
	for _, e := range examples {
		ew = max(ew, len(e[0]))
	}
	for _, e := range examples {
		fmt.Fprintf(w, "  %s%s  %s\n", st.cmd.Render(e[0]), strings.Repeat(" ", ew-len(e[0])), st.dim.Render(e[1]))
	}
	fmt.Fprintln(w)
	head("Settings")
	fmt.Fprintln(w, "  "+cfgPath)
}
