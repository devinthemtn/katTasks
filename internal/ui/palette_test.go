package ui

import "testing"

func TestFuzzyScoreOrder(t *testing.T) {
	q := "th"
	start, word, inside, sub := fuzzyScore(q, "theme_picker"), fuzzyScore(q, "cycle_theme"), fuzzyScore(q, "toggle_show_done"), fuzzyScore("tsd", "toggle_show_done")
	if !(start > word && word > inside && inside > sub && sub > 0) {
		t.Errorf("scores start=%d word=%d inside=%d sub=%d", start, word, inside, sub)
	}
	if fuzzyScore("zzz", "quit") != 0 {
		t.Error("non-match scored")
	}
	if fuzzyScore("show done", "toggle_show_done") <= 0 {
		t.Error("space should match underscore")
	}
}
