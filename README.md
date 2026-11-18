# katTasks

A terminal UI for [todo.txt](https://github.com/todotxt/todo.txt) files, built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea), Bubbles and Lip Gloss.

## Install

Requires Go 1.24 or newer.

    go install github.com/devinthemtn/katTasks@latest

This puts `katTasks` in `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure
that's on your `PATH`.

Or build from a checkout:

    go build -o katTasks .

## Config

On first run a commented config is written to `~/.config/katTasks/config.toml`
(or `$XDG_CONFIG_HOME/katTasks/config.toml`; override with `--config path`).

| key                 | default      | meaning                                         |
|---------------------|--------------|-------------------------------------------------|
| `todo_file`         | `~/todo.txt` | the todo.txt file                               |
| `done_file`         | `""`         | archive file; empty = `done.txt` next to todo   |
| `add_creation_date` | `true`       | prefix new tasks with today's date              |
| `auto_archive`      | `false`      | move tasks to done.txt as soon as they're done  |
| `hide_completed`    | `false`      | start with completed tasks hidden               |
| `default_sort`      | `priority`   | `priority`, `due`, `created` or `file`          |

## Keys

| key            | action                         | key   | action                     |
|----------------|--------------------------------|-------|----------------------------|
| `j`/`k`        | move                           | `a`   | add (raw todo.txt line)    |
| `g`/`G`        | top / bottom                   | `e`   | edit                       |
| `ctrl+d/u`     | half page                      | `x`   | toggle done                |
| `tab`, `h`/`l` | switch sidebar / tasks         | `d`   | delete                     |
| `/`            | filter text (`esc` clears)     | `p`   | set priority (then A–Z)    |
| `s`            | cycle sort                     | `+/-` | raise / lower priority     |
| `H`            | hide / show completed          | `u`   | undo                       |
| `A`            | archive completed to done.txt  | `r`   | reload from disk           |
| `?`            | help                           | `q`   | quit                       |

The sidebar filters by project (`+foo`), context (`@bar`) or "has due date".
When a project or context is selected, `a` pre-fills it.

While typing, `tab` completes `+project`, `@context` and `due:` words.
Relative due dates are converted to `YYYY-MM-DD` on save: `due:today`,
`due:tomorrow`, `due:fri`, `due:+3d`, `due:+2w`, `due:+1m`.

### Recurring tasks

Add a `rec:` tag and completing the task queues the next occurrence right
after it (marked `↻` in the list):

| tag        | next occurrence                                   |
|------------|---------------------------------------------------|
| `rec:1w`   | 1 week after the day you complete it              |
| `rec:+1w`  | 1 week after the current due date (strict)        |
| `rec:3d`   | units: `d` days, `b` business days, `w` weeks,    |
| `rec:2m`   | `m` months, `y` years                             |

If the task has no due date, one is added. A `t:` threshold date moves
along with the due date, keeping the same gap. Month steps clamp to the end
of the month (Jan 31 + 1m = Feb 28).

Completing a task drops its priority (todo.sh convention) but keeps it as
`pri:X` so un-completing restores it.

katTasks never overwrites changes made by other programs: if todo.txt changed
on disk since it was loaded, edits are refused until you press `r`.
