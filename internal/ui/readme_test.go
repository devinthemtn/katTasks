package ui

import (
	"os"
	"strings"
	"testing"
)

// The README's key table must mention every action, so it can't fall
// behind the code.
func TestReadmeListsActions(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skip(err)
	}
	for _, a := range actions {
		if !strings.Contains(string(b), "`"+a.name+"`") {
			t.Errorf("README.md doesn't mention action %s", a.name)
		}
	}
}
