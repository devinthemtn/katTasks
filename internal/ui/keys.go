package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up, Down, Top, Bottom, HalfUp, HalfDown key.Binding
	SwitchPane, Left, Right                 key.Binding
	Add, Edit, Toggle, Delete               key.Binding
	Priority, PriUp, PriDown                key.Binding
	Filter, ClearFilter, Sort, HideDone     key.Binding
	Archive, Undo, Reload, Help, Quit       key.Binding
}

func newKeyMap() keyMap {
	b := func(keys []string, help, desc string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
	}
	return keyMap{
		Up:          b([]string{"k", "up"}, "k/↑", "up"),
		Down:        b([]string{"j", "down"}, "j/↓", "down"),
		Top:         b([]string{"g", "home"}, "g", "top"),
		Bottom:      b([]string{"G", "end"}, "G", "bottom"),
		HalfUp:      b([]string{"ctrl+u", "pgup"}, "ctrl+u", "page up"),
		HalfDown:    b([]string{"ctrl+d", "pgdown"}, "ctrl+d", "page down"),
		SwitchPane:  b([]string{"tab", "shift+tab"}, "tab", "switch pane"),
		Left:        b([]string{"h", "left"}, "h", "sidebar"),
		Right:       b([]string{"l", "right"}, "l", "tasks"),
		Add:         b([]string{"a", "n"}, "a", "add"),
		Edit:        b([]string{"e", "enter"}, "e", "edit"),
		Toggle:      b([]string{"x", " "}, "x", "toggle done"),
		Delete:      b([]string{"d"}, "d", "delete"),
		Priority:    b([]string{"p"}, "p", "set priority"),
		PriUp:       b([]string{"+", "="}, "+/-", "raise/lower priority"),
		PriDown:     b([]string{"-"}, "-", "lower priority"),
		Filter:      b([]string{"/"}, "/", "filter"),
		ClearFilter: b([]string{"esc"}, "esc", "clear filter"),
		Sort:        b([]string{"s"}, "s", "cycle sort"),
		HideDone:    b([]string{"H"}, "H", "hide/show done"),
		Archive:     b([]string{"A"}, "A", "archive done"),
		Undo:        b([]string{"u"}, "u", "undo"),
		Reload:      b([]string{"r"}, "r", "reload file"),
		Help:        b([]string{"?"}, "?", "help"),
		Quit:        b([]string{"q", "ctrl+c"}, "q", "quit"),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Add, k.Edit, k.Toggle, k.Priority, k.Filter, k.Sort, k.SwitchPane, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom, k.HalfUp, k.HalfDown, k.SwitchPane},
		{k.Add, k.Edit, k.Toggle, k.Delete, k.Priority, k.PriUp, k.Undo},
		{k.Filter, k.ClearFilter, k.Sort, k.HideDone, k.Archive, k.Reload, k.Quit},
	}
}
