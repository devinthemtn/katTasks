# katTasks

A keyboard-driven terminal UI for [todo.txt](https://github.com/todotxt/todo.txt)
files, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), Bubbles
and Lip Gloss. Your tasks stay in plain todo.txt, readable and editable by any
other tool.

```
katTasks [flags]                  open the task list
katTasks [flags] COMMAND ARGS...  run one command and exit (see Command line)
```

- A sidebar of projects, contexts and saved searches, each with a count of open tasks.
- Vim-style keys, two-key chords, visual multi-select, and a command palette (`:`).
- Every key can be rebound, and you can pick or write your own color theme.
- A search language: `+Work due:+1w -@waiting pri:A-C`.
- `rec:` recurring tasks, `due:` dates you can type as `due:fri`, and archiving to `done.txt`.
- todo.sh-style commands (`katTasks add …`, `katTasks ls …`, `katTasks do 3`), with `-json` output.

## Install

Requires Go 1.24 or newer.

    go install github.com/devinthemtn/katTasks@latest

This puts `katTasks` in `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure
that's on your `PATH`. Or build from a checkout with `go build -o katTasks .`.

Prebuilt binaries for Linux, macOS and Windows (x86-64 and ARM64) are on the
[releases page](https://github.com/devinthemtn/katTasks/releases).

## Flags

| flag | |
|---|---|
| `-file PATH` | open this todo.txt instead of `files.todo` |
| `-config PATH` | use another settings file |
| `-makeconfig` | write the settings file with every default, then exit (never overwrites) |
| `-updateconfig` | add settings your file is missing, for example ones new in this version, as commented-out lines with their explanations, then exit. Your own settings and comments are left alone |
| `-maketheme NAME` | write the current theme to `themes/NAME.toml` to edit, then exit |
| `-json`, `-f` | for commands: print JSON; delete without asking |
| `-version`, `-h` | print the version / help (help includes the settings path) |

## Settings

Settings live in `~/.config/katTasks/config.toml`, or more generally your OS
config folder plus `katTasks/config.toml`; `katTasks -h` prints the path.
katTasks creates the file with the defaults on first start and never
overwrites it. Every setting is optional. Press `ctrl+r` to apply edits.
Mistakes are shown in the footer until fixed, and the default is used for
that setting.

```toml
theme = "default"            # default | terminal | nord | gruvbox | a file in themes/

[files]
todo = "~/todo.txt"          # ~ and $VARS are expanded
done = ""                    # empty = done.txt next to todo

[tasks]
add_creation_date = true     # prefix new tasks with today's date
auto_archive = false         # move tasks to done as soon as they're completed

[view]
hide_completed = false       # H toggles
sort = "priority"            # priority | due | created | file (s cycles)
line_numbers = false         # L toggles; these are the numbers the commands use
hide_keys = []               # key:value tags to hide from rows, e.g. ["uid"]

[clipboard]
command = ""                 # also pipe copies to this, e.g. "wl-copy" or "clip.exe"

[keys]                       # action = "key" or ["key", "key"]
# add = ["a", "n"]
# top = "gg"

[filters]                    # saved searches: name = "query"
# work-week = "+Work due:+1w"
```

## Keys

These are the defaults. The `?` screen and the `:` palette always show your
current bindings. Two plain characters, such as `yy`, are a chord: press the
keys one after the other, within 600 ms.

| action | keys | does |
|---|---|---|
| `down` / `up` | `j` `k`, arrows | move |
| `top` / `bottom` | `g` `home` / `G` `end` | first / last task |
| `half_page_down` / `half_page_up` | `ctrl+d` `pgdown` / `ctrl+u` `pgup` | half a page |
| `switch_pane` | `tab` | sidebar ⇄ tasks |
| `focus_sidebar` / `focus_tasks` | `h` / `l`, arrows | go to the sidebar / tasks |
| `add` | `a` `n` | add a task, typed as a todo.txt line |
| `edit` | `e` `enter` | edit the whole line |
| `toggle_done` | `x` `space` | complete / reopen |
| `delete` | `d` | delete (asks first) |
| `set_priority` | `p` | then a letter; `space` clears |
| `priority_up` / `priority_down` | `+` `=` / `-` | raise / lower priority |
| `move_down` / `move_up` | `J` / `K` | reorder a task among tasks that tie under the current sort (any task in `file` sort) |
| `visual` | `v` | visual mode: `space` marks rows; done, delete, priority and copy then apply to all marked |
| `select_all` | `V` | mark every task shown |
| `copy_line` / `copy_body` | `yy` / `yb` | copy the line / just the words, without priority, dates, `+` `@` or tags |
| `undo` | `u` | undo (50 steps; a bulk change is one step) |
| `archive_done` | `A` | move completed tasks to done.txt |
| `filter` | `/` | search; see below |
| `save_search` | `fs` | save the current search under a name |
| `saved_searches` | `ff` | jump to saved searches in the sidebar |
| `clear_filter` | `esc` | leave visual mode, then clear the search, then the sidebar filter |
| `cycle_sort` | `s` | priority → due → created → file order |
| `toggle_show_done` | `H` | hide / show completed tasks |
| `toggle_line_numbers` | `L` | show task numbers |
| `theme_picker` | `T` | pick a theme, previewing as you move |
| `cycle_theme` | (none) | switch to the next theme |
| `palette` | `:` `ctrl+p` | fuzzy-search every action, and your saved searches |
| `reload` | `r` | re-read todo.txt from disk |
| `reload_config` | `ctrl+r` | re-read the settings |
| `help` | `?` | all keys |
| `quit` | `q` | quit (`ctrl+c` always quits) |

To rebind an action, put it in `[keys]`. A key you bind there stops doing its
default action, and `[]` unbinds an action:

```toml
[keys]
top = "gg"                   # vim-style; g alone is then free
toggle_done = "x"            # space no longer completes tasks
add = ["n", "ctrl+n"]
```

Keys are written like `ctrl+u`, `alt+x`, `shift+tab`, `enter`, `esc`, `space`,
`tab`, `up`, `pgdown` or `f5`.

Adding (`a`) or editing a task opens a small box in the middle of the screen,
with a reminder of the `(A)`, `+project`, `@context`, `due:` and `rec:` syntax
underneath. While typing a task, `tab` completes `+project`, `@context`, `due:` and `rec:`
words. Relative due dates are converted to `YYYY-MM-DD` when you save:
`due:today`, `due:tomorrow`, `due:fri`, `due:+3d`, `due:+2w`, `due:+1m`. When a
project or context is selected in the sidebar, `a` fills it in for you.

## Search

`/` filters as you type. A task has to match every term:

| term | matches |
|---|---|
| `milk` | tasks containing "milk". Smartcase: case-insensitive unless you type an uppercase letter |
| `"buy milk"` | a phrase |
| `+Work`, `@phone` | tasks with exactly that project / context |
| `due:+1w` | due between today and a week from now (`due:1w` is the same) |
| `due:-3d` | due in the last 3 days. Units: `d`, `b` (business days), `w`, `m`, `y` |
| `due:today`, `due:tomorrow`, `due:2026-12-01` | due on that day |
| `due:overdue`, `due:none` | past due and not done / no due date |
| `pri:A`, `pri:A-C`, `pri:none` | by priority |
| `-term` | negates any term, e.g. `-@waiting`, `-due:none` |

Matching words are highlighted. `fs` saves the current search (or the selected
project or context) to `[filters]`. Saved searches appear in the sidebar under
SAVED and in the palette.

## Recurring tasks

Add a `rec:` tag, and completing the task queues the next occurrence right
after it (marked `↻` in the list):

| tag | next occurrence |
|---|---|
| `rec:1w` | 1 week after the day you complete it |
| `rec:+1w` | 1 week after the current due date (strict); good for bills |
| `rec:3d`, `rec:5b`, `rec:2m`, `rec:1y` | units: `d` days, `b` business days, `w` weeks, `m` months, `y` years |

If the task has no due date, one is added. A `t:` threshold date moves along
with the due date and keeps the same gap. Month steps stop at the end of the
month, so Jan 31 plus a month is Feb 28.

Completing a task drops its priority, as todo.sh does, but keeps it as a
`pri:X` tag so reopening the task restores it.

## Themes

Built in: `default`, which adapts to light and dark terminals; `terminal`,
which uses only your terminal's 16 colors; `nord`; and `gruvbox`. Press `T` to
pick one. The choice is saved to `theme` in config.toml.

Any `themes/NAME.toml` next to config.toml adds a theme called `NAME`. If it
has the same name as a built-in, it replaces it. `katTasks -maketheme NAME`
writes the current theme there with every color explained. A theme only needs
the colors it changes; the rest come from `base`:

```toml
base = "nord"
accent = "#ff79c6"
cursor_bg = { light = "#eeeeee", dark = "#333344" }   # differs on light / dark terminals
```

A color is `"#rrggbb"`, an ANSI number from `"0"` to `"255"`, or `""` for the
terminal's own color. The colors are `fg dim accent border border_focus
cursor_bg selected_bg match_bg title_fg title_bg project context tag pri_a
pri_b pri_c pri_d pri_other due today overdue done ok error`. A theme file with
a mistake is skipped, with a warning in the footer.

## Command line

Put flags before the command. `N` is the task's line in todo.txt; press `L` in
the task list to see the numbers. Several numbers can be given as `do 2 5` or
`do 2,5`.

| command | |
|---|---|
| `add TEXT…` (`a`) | add a task; `due:fri` and similar are resolved |
| `append N TEXT…` (`app`), `prepend N TEXT…` (`prep`) | add text to the end / start of task N |
| `replace N TEXT…` | replace task N; its priority and creation date are kept if TEXT has none |
| `pri N X` (`p`), `depri N…` (`dp`) | set / remove the priority |
| `do N…` (`done`) | complete tasks; `rec:` tasks queue their next copy |
| `del N [TERM]` (`rm`) | delete task N (asks unless `-f`), or just remove TERM from it |
| `archive` | move completed tasks to done.txt |
| `ls [QUERY…]` (`list`) | list tasks matching a search query, as in `/` |
| `lsa [QUERY…]` (`listall`) | the same, including done.txt (numbered 0) |
| `lsp [X or A-C]` (`listpri`) | tasks with a priority |
| `lsprj` (`listproj`), `lsc` (`listcon`) | every project / context |

```
$ katTasks add "Pay rent +Home due:2026-12-01 rec:+1m"
4 2026-11-18 Pay rent +Home due:2026-12-01 rec:+1m
TODO: 4 added.
$ katTasks ls +Home due:+1m
$ katTasks -json ls pri:A | jq '.[].text'
$ katTasks -f del 4
```

With `-json`, lists print an array of tasks
(`n raw done priority created completed text projects contexts tags due`).
Changes print `{action, tasks, added, message}`.

## Safety

- Every save writes a temporary file and renames it over todo.txt, so a crash can't leave the file half-written.
- If another program changes todo.txt while katTasks is open, edits are refused until you press `r` to reload. katTasks never overwrites those changes.
- Archiving writes done.txt before removing anything from todo.txt.

## Releasing

Releases are built by [GoReleaser](https://goreleaser.com) in GitHub Actions
(`.github/workflows/release.yml`, configured by `.goreleaser.yaml`). Pushing a
tag runs the tests, builds every platform, and publishes the release with
archives, checksums and a changelog from the commits:

    git tag -a v0.3.0 -m "katTasks 0.3.0"
    git push origin v0.3.0

The tag is the version: `katTasks -version` reports it, and there is nothing to
bump in the code. Edit the generated notes on GitHub afterwards if you like. To
try the build without publishing, run the workflow by hand (Actions → release
→ Run workflow); the archives are attached to the run. Locally:
`go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean`.
