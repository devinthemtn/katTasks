package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsMatchFile(t *testing.T) {
	c, errs := Parse(DefaultSettingsFile)
	if len(errs) != 0 {
		t.Fatalf("default.toml has problems: %v", errs)
	}
	if c.Theme != "default" || c.View.Sort != "priority" || !c.Tasks.AddCreationDate {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if filepath.Base(c.Files.Done) != "done.txt" || filepath.Dir(c.Files.Done) != filepath.Dir(c.Files.Todo) {
		t.Errorf("done = %q, todo = %q", c.Files.Done, c.Files.Todo)
	}
}

func TestDoneFollowsTodo(t *testing.T) {
	c, errs := Parse("[files]\ntodo = \"/x/y/todo.txt\"\n")
	if len(errs) != 0 || c.Files.Done != "/x/y/done.txt" {
		t.Fatalf("done = %q, errs %v", c.Files.Done, errs)
	}
}

func TestParseReportsAndResets(t *testing.T) {
	c, errs := Parse(`
bogus = 1
[view]
sort = "random"
line_numbers = true
[keys]
add = "n"
top = ["gg", "home"]
quit = 5
[filters]
week = "due:+1w"
`)
	msgs := ""
	for _, e := range errs {
		msgs += e.Error() + "\n"
	}
	for _, want := range []string{`unknown setting "bogus"`, `view.sort`, `keys.quit`} {
		if !strings.Contains(msgs, want) {
			t.Errorf("missing error %q in:\n%s", want, msgs)
		}
	}
	if c.View.Sort != "priority" || !c.View.LineNumbers {
		t.Errorf("view = %+v", c.View)
	}
	if got := c.Keys["add"]; len(got) != 1 || got[0] != "n" {
		t.Errorf("keys.add = %v", got)
	}
	if got := c.Keys["top"]; len(got) != 2 || got[0] != "gg" {
		t.Errorf("keys.top = %v", got)
	}
	if _, ok := c.Keys["quit"]; ok {
		t.Error("bad keys.quit kept")
	}
	if c.Filters["week"] != "due:+1w" {
		t.Errorf("filters = %v", c.Filters)
	}
}

func TestSyntaxErrorGivesDefaults(t *testing.T) {
	c, errs := Parse("theme = \n")
	if len(errs) != 1 || c.Theme != "default" {
		t.Fatalf("got %+v %v", c.Theme, errs)
	}
}

func TestSetValue(t *testing.T) {
	base := "# top comment\ntheme = \"default\"\n\n[view]\n# sort comment\nsort = \"priority\"\n\n# about filters\n[filters]\n# example = \"x\"\n"

	got := SetValue(base, "", "theme", `"nord"`)
	if !strings.Contains(got, "theme = \"nord\"\n") || strings.Contains(got, `"default"`) || !strings.Contains(got, "# top comment") {
		t.Errorf("replace top-level:\n%s", got)
	}

	got = SetValue(base, "view", "line_numbers", "true")
	if !strings.Contains(got, "sort = \"priority\"\nline_numbers = true\n\n# about filters") {
		t.Errorf("insert into table:\n%s", got)
	}

	got = SetValue(base, "filters", "work week", `"+Work"`)
	if !strings.HasSuffix(got, "# example = \"x\"\n\"work week\" = \"+Work\"\n") {
		t.Errorf("insert quoted key into last table:\n%s", got)
	}
	again := SetValue(got, "filters", "work week", `"+Home"`)
	if strings.Count(again, "work week") != 1 || !strings.Contains(again, `"+Home"`) {
		t.Errorf("replace quoted key:\n%s", again)
	}

	got = SetValue(base, "clipboard", "command", `"wl-copy"`)
	if !strings.HasSuffix(got, "\n\n[clipboard]\ncommand = \"wl-copy\"\n") {
		t.Errorf("append table:\n%s", got)
	}

	// Result must stay valid TOML that Parse accepts.
	c, errs := Parse(SetValue(DefaultSettingsFile, "filters", `odd "name"`, String(`say "hi" \ there`)))
	if len(errs) != 0 || c.Filters[`odd "name"`] != `say "hi" \ there` {
		t.Errorf("round trip: %v %v", c.Filters, errs)
	}
}

func TestUpdateAddsMissing(t *testing.T) {
	out, added, err := UpdateText("theme = \"nord\"\n\n[view]\nsort = \"due\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) == 0 || !strings.Contains(out, "# line_numbers = false") || !strings.Contains(out, "[files]") {
		t.Errorf("added %v:\n%s", added, out)
	}
	if !strings.Contains(out, "theme = \"nord\"") || !strings.Contains(out, "sort = \"due\"") {
		t.Error("user settings lost")
	}
	if _, again, _ := UpdateText(out); len(again) != 0 {
		t.Errorf("second update added %v", again)
	}
}

func TestLoadCreates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.toml")
	_, created, errs := Load(p)
	if !created || len(errs) != 0 {
		t.Fatalf("created=%v errs=%v", created, errs)
	}
	if b, _ := os.ReadFile(p); string(b) != DefaultSettingsFile {
		t.Error("written file differs from default")
	}
	if err := Save(p, "", "theme", `"gruvbox"`); err != nil {
		t.Fatal(err)
	}
	c, _, _ := Load(p)
	if c.Theme != "gruvbox" {
		t.Errorf("theme = %q after Save", c.Theme)
	}
}

func TestDefaultHidesCompleted(t *testing.T) {
	if !Defaults().View.HideCompleted {
		t.Error("completed tasks should be hidden by default")
	}
	c, errs := Parse("[view]\nsort = \"due\"\n")
	if len(errs) != 0 || !c.View.HideCompleted {
		t.Errorf("a config without hide_completed should hide them: %v", errs)
	}
}

func TestParseGUI(t *testing.T) {
	if d := Defaults(); d.GUI.FontFamily != "" || d.GUI.FontSize != DefaultFontSize {
		t.Errorf("defaults: %+v", d.GUI)
	}
	c, errs := Parse("[gui]\nfont_family = \"  Fira Code, monospace \"\nfont_size = 18\n")
	if len(errs) > 0 || c.GUI.FontFamily != "Fira Code, monospace" || c.GUI.FontSize != 18 {
		t.Errorf("valid: %+v %v", c.GUI, errs)
	}
	c, errs = Parse("[gui]\nfont_family = \"x; color: red\"\nfont_size = 2\n")
	if len(errs) != 2 || c.GUI.FontFamily != "" || c.GUI.FontSize != DefaultFontSize {
		t.Errorf("invalid: %+v %v", c.GUI, errs)
	}
}

// The README's settings block must show every setting in default.toml.
func TestReadmeListsSettings(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skip(err)
	}
	readme := string(b)
	start := strings.Index(readme, "## Settings")
	end := strings.Index(readme[start:], "## Keys")
	block := readme[start : start+end]
	table := ""
	for _, line := range strings.Split(DefaultSettingsFile, "\n") {
		switch {
		case strings.HasPrefix(line, "["):
			table = strings.Trim(line, "[]")
			if !strings.Contains(block, line) {
				t.Errorf("README settings lack %s", line)
			}
		case len(line) > 0 && line[0] >= 'a' && line[0] <= 'z' && strings.Contains(line, " = "):
			key, _, _ := strings.Cut(line, " ")
			if !strings.Contains(block, "\n"+key+" ") {
				t.Errorf("README settings lack %s.%s", table, key)
			}
		}
	}
}
