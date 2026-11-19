package query

import (
	"strings"
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// Wednesday 2026-11-18.
var now = time.Date(2026, 11, 18, 15, 0, 0, 0, time.Local)

func TestMatch(t *testing.T) {
	tasks := map[string]string{
		"mom":   "(A) 2026-11-10 Call Mom +Family @phone due:2026-11-18",
		"rep":   "(B) Write report +Work @desk due:2026-11-15",
		"milk":  "Buy milk +Home @errands",
		"bike":  "(C) Fix bike +Home due:2026-11-21",
		"trip":  "Plan trip +Family due:2026-12-20",
		"rent":  "x 2026-11-17 Pay rent +Home due:2026-11-01",
		"phone": "(D) Phone bill +HomeOffice due:2026-11-25",
	}
	cases := []struct {
		q    string
		want []string
	}{
		{"", []string{"mom", "rep", "milk", "bike", "trip", "rent", "phone"}},
		{"+Home", []string{"milk", "bike", "rent"}},
		{"+home", []string{"milk", "bike", "rent"}},
		{"-+Home", []string{"mom", "rep", "trip", "phone"}},
		{"phone", []string{"mom", "phone"}},
		{"Phone", []string{"phone"}},
		{"@phone", []string{"mom"}},
		{"due:+1w", []string{"mom", "bike", "phone"}},
		{"due:1w", []string{"mom", "bike", "phone"}},
		{"due:3d", []string{"mom", "bike"}},
		{"due:2b", []string{"mom"}},
		{"due:-3d", []string{"mom", "rep"}},
		{"due:-1m", []string{"mom", "rep", "rent"}},
		{"due:today", []string{"mom"}},
		{"due:tomorrow", nil},
		{"due:2026-11-21", []string{"bike"}},
		{"due:overdue", []string{"rep"}},
		{"due:none", []string{"milk"}},
		{"pri:A", []string{"mom"}},
		{"pri:a-c", []string{"mom", "rep", "bike"}},
		{"pri:none", []string{"milk", "trip", "rent"}},
		{"+Home pri:none -rent", []string{"milk"}},
		{`"call mom"`, []string{"mom"}},
		{`-"call mom" +Family`, []string{"trip"}},
		{"due:bogus", nil}, // plain text: nobody has it
	}
	for _, c := range cases {
		q := Parse(c.q)
		got := map[string]bool{}
		for name, line := range tasks {
			if q.Match(todo.Parse(line), now) {
				got[name] = true
			}
		}
		want := map[string]bool{}
		for _, n := range c.want {
			want[n] = true
		}
		for n := range tasks {
			if got[n] != want[n] {
				t.Errorf("%q: %s matched=%v, want %v", c.q, n, got[n], want[n])
			}
		}
	}
}

func TestHighlights(t *testing.T) {
	q := Parse(`milk +Home -bread due:+1w "call mom"`)
	for w, want := range map[string]bool{"milk": true, "Milkshake": true, "+Home": true, "bread": false, "due:2026-11-20": false, "Call": true, "Mom": true, "+Work": false} {
		if q.Highlights(w) != want {
			t.Errorf("Highlights(%q) = %v", w, !want)
		}
	}
}

func TestRelativeDayUsesMatchTime(t *testing.T) {
	task := todo.Parse("x due:2026-11-19")
	if !Parse("due:today").Match(task, now.AddDate(0, 0, 1)) || !Parse("due:tomorrow").Match(task, now) {
		t.Error("due:today/tomorrow should follow the time given to Match")
	}
}

func TestStatusProgress(t *testing.T) {
	tasks := map[string]string{
		"a": "draft status:doing progress:40",
		"b": "review status:blocked",
		"c": "ship progress:100%",
		"d": "plain",
		"e": "odd progress:lots",
	}
	cases := []struct {
		q    string
		want []string
	}{
		{"status:doing", []string{"a"}},
		{"status:DOING", []string{"a"}},
		{"status:any", []string{"a", "b"}},
		{"status:none", []string{"c", "d", "e"}},
		{"-status:doing", []string{"b", "c", "d", "e"}},
		{"progress:40", []string{"a"}},
		{"progress:>40", []string{"c"}},
		{"progress:<41", []string{"a"}},
		{"progress:30-100", []string{"a", "c"}},
		{"progress:100-30", []string{"a", "c"}},
		{"progress:any", []string{"a", "c"}},
		{"progress:none", []string{"b", "d", "e"}},
	}
	for _, c := range cases {
		q := Parse(c.q)
		var got []string
		for _, k := range []string{"a", "b", "c", "d", "e"} {
			if q.Match(todo.Parse(tasks[k]), now) {
				got = append(got, k)
			}
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%q: got %v, want %v", c.q, got, c.want)
		}
	}
}
