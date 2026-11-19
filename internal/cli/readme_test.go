package cli

import (
	"os"
	"strings"
	"testing"
)

// The README's command table must mention every command and alias, so it
// can't fall behind the code.
func TestReadmeListsCommands(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Skip(err)
	}
	readme := string(b)
	for _, c := range commands {
		for _, n := range c.names {
			if !strings.Contains(readme, "`"+n+"`") && !strings.Contains(readme, "`"+n+" ") {
				t.Errorf("README.md doesn't mention command %s", n)
			}
		}
	}
}
