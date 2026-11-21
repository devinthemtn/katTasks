package query

import (
	"testing"
	"time"

	"github.com/devinthemtn/katTasks/internal/todo"
)

// Any search parses and matches without panicking or hanging.
func FuzzQuery(f *testing.F) {
	for _, s := range []string{
		"+Work due:+1w -@waiting pri:A-C status:doing",
		`"two words" -"not this"`,
		"due:overdue due:none due:today due:2026-12-01 due:-3d due:+999999999b due:-999999999b",
		"pri:none pri:Z-A pri:", "progress:>50 progress:<50 progress:25-75 progress:any",
		"status:any status:none -", `"`, `-"`, "+ @ due: pri:- progress:-",
	} {
		f.Add(s)
	}
	now := time.Date(2026, 3, 4, 9, 0, 0, 0, time.Local)
	tasks := []*todo.Task{
		todo.Parse("(A) 2026-01-02 report +Work @desk due:2026-03-05 status:doing progress:40"),
		todo.Parse("x 2026-03-01 done thing pri:B"),
		todo.Parse("plain words"),
		todo.Parse(""),
	}
	f.Fuzz(func(t *testing.T, q string) {
		p := Parse(q)
		for _, task := range tasks {
			p.Match(task, now)
		}
		p.Empty()
	})
}
