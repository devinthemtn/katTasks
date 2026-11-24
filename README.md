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
- A search language: `+Work due:+1w -@waiting pri:A-C status:doing`.
- `rec:` recurring tasks, `due:` dates you can type as `due:fri`, and archiving to `done.txt`.
- A month calendar and a week agenda, with times of day from `at:` tags.
- Tasks in progress (`status:doing`, `progress:40`), shown as badges in the list.
- Alerts at a task's `at:` time and alarms that ring like an alarm clock,
  with a background reminder for when katTasks is closed.
- Lua scripts, run from `:` or as `/name` while typing a task.
- todo.sh-style commands (`katTasks add …`, `katTasks ls …`, `katTasks do 3`,
  `katTasks alarm +5m Tea is ready`), with `-json` output.
- A desktop app, katTasks-gui, with the same keys plus the mouse.

## Install

Requires Go 1.24 or newer.

    go install github.com/devinthemtn/katTasks@latest

This puts `katTasks` in `$(go env GOPATH)/bin` (usually `~/go/bin`); make sure
that's on your `PATH`. Or build from a checkout with `go build -o katTasks .`.

Prebuilt binaries for Linux, macOS and Windows (x86-64 and ARM64) are on the
[releases page](https://github.com/devinthemtn/katTasks/releases). On
Windows, `katTasks-<version>-windows-amd64-setup.exe` there installs both
the terminal app and the desktop app.

## Desktop app (katTasks-gui)

`katTasks-gui` is the same task list in a window, for Linux, macOS and
Windows. It runs the terminal app's own list: the keys, chords, `[keys]`
overrides, themes, saved searches, Lua scripts and messages are all the same,
and it reads the same settings file and todo.txt. It also adds:

- **Mouse:** click a task to select it, double-click to edit it, click its
  `[ ]` to mark it done; click a sidebar entry to show it; scroll with the
  wheel; click a command, theme or completion to choose it; click a key in the
  footer or on the help screen to run it.
- **Calendar:** each day lists its tasks. Click a day to choose it,
  double-click to list its tasks; click a task to go to it, double-click to
  edit it; `‹` `›` change the month.
- **Font:** `font_family` and `font_size` under `[gui]` in the settings set
  the window's font (see [Settings](#settings)); `ctrl+r` applies edits.
  `ctrl+=` and `ctrl+-` zoom for the session, and `ctrl+0` returns to
  `font_size`.
- **Paste:** `ctrl+v` pastes into the task being typed.
- **Clipboard:** `yy` and `yb` use the system clipboard, so `clipboard.command`
  isn't needed.

`ctrl+c` doesn't quit, as it would in a terminal; use `q` or close the window.
It takes `-file`, `-config` and `-version` (or `-v`) like `katTasks`.

The app is built with [Wails 3](https://v3.wails.io) and lives in
`katTasks-gui/`, as its own Go module, because Wails needs a newer Go (1.26)
than katTasks itself. To build it you need Go 1.26, Node.js and npm, `task`
and `wails3` (`CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`;
without `CGO_ENABLED=0` it needs GTK 4 on Linux); on
Linux also the GTK 3 and WebKitGTK 4.1 development packages
(`libgtk-3-dev libwebkit2gtk-4.1-dev` on Debian and Ubuntu). Then, from the
top of the repository:

    task install               # build both apps and install them into $(go env GOPATH)/bin
    task                       # just build: bin/katTasks and katTasks-gui/bin/katTasks-gui
    task build-windows         # bin/katTasks.exe and katTasks-gui/bin/katTasks-gui.exe, from any OS
    task build-win-installer   # a Windows installer, bin/katTasks-<version>-windows-amd64-setup.exe (needs makensis)
    task release               # on a tagged commit: the release files, in bin/release-<version>/
    task dev-gui               # run the app with live reload while working on it
    task test                  # every Go test, and the window's key-handling tests

`task install` takes `BINDIR=…` to install somewhere else. On Linux and macOS
it also adds lowercase `kattasks` and `kattasks-gui` links, so either
spelling works. On Linux it also adds katTasks to the app launcher with its
icon (`task install-launcher`,
in `~/.local/share`), which is also how the taskbar or dock finds the icon. In
WSL it skips that and installs the Windows versions instead
(`task install-windows`):

- `katTasks.exe` and `katTasks-gui.exe` go into
  `%LOCALAPPDATA%\Programs\katTasks`, which is added to your Windows PATH;
- a **katTasks** Start Menu shortcut, with the app's icon;
- `katTasks-win` in `BINDIR`, which starts the Windows app from the WSL shell;
- the Windows settings' `files.todo` is pointed at your WSL todo.txt
  (`\\wsl.localhost\…`), so both share one task list. This only happens while
  the Windows setting is still the default and that todo.txt is empty, so a
  choice made on Windows is never overwritten;
- a running Windows `katTasks remind` (see [Alerts](#alerts)) is stopped for
  the copy and started again, since Windows locks a running program.

The Windows installer installs the same two programs, the Start Menu shortcut
and PATH entry, per user (no admin), with an uninstaller under Settings → Apps.
It also starts the background reminder and sets it to start at sign-in.

## Flags

| flag | |
|---|---|
| `-file PATH` | open this todo.txt instead of `files.todo` |
| `-config PATH` | use another settings file |
| `-make-config` | write the settings file with every default, then exit (never overwrites) |
| `-update-config` | add settings your file is missing, for example ones new in this version, as commented-out lines with their explanations, then exit. Your own settings and comments are left alone |
| `-make-theme NAME` | write the current theme to `themes/NAME.toml` to edit, then exit |
| `-paths` | print the settings file, todo.txt and done.txt in use, then exit |
| `-json`, `-f` | for commands: print JSON; delete without asking |
| `-version` (or `-v`), `-h` | print the version / help, which ends with examples and the settings path. `katTasks help` does the same. It's in color in a terminal (`NO_COLOR=1` turns that off) and plain when piped, so `katTasks -h \| less` works |

## Settings

Settings live in `~/.config/katTasks/config.toml`, or more generally your OS
config folder plus `katTasks/config.toml`; `katTasks -h` prints the path.
katTasks creates the file with the defaults on first start and never
overwrites it. Every setting is optional. Press `ctrl+r` to apply edits.
Mistakes are shown in the footer until fixed, and the default is used for
that setting.

```toml
theme = "default"            # default | terminal | nord | gruvbox | dracula | a file in themes/

[files]
todo = "~/todo.txt"          # ~ and $VARS are expanded
done = ""                    # empty = done.txt next to todo

[tasks]
add_creation_date = true     # prefix new tasks with today's date
auto_archive = false         # move tasks to done as soon as they're completed

[view]
hide_completed = true        # H toggles
sort = "priority"            # priority | due | created | file (s cycles)
line_numbers = false         # L toggles; these are the numbers the commands use
hide_keys = []               # key:value tags to hide from rows, e.g. ["uid"]

[clipboard]
command = ""                 # also pipe copies to this, e.g. "wl-copy" or "clip.exe"

[gui]                        # katTasks-gui only
font_family = ""             # e.g. "Fira Code" or "Iosevka, Menlo"; empty = a built-in monospace list
font_size = 14               # pixels, 8–48; ctrl+= / ctrl+- zoom, ctrl+0 returns to this

[alerts]                     # see Alerts below
enabled = true               # notify at a task's at: time
lead = "10m"                 # this long before it; "0m" at the time
popup = true                 # a desktop notification
sound = true                 # and a sound
sound_file = ""              # empty = the system's notification sound; .wav works everywhere

[alarms]                     # see Alarms below
enabled = true               # ring at a task's alarm: date and time
popup = true                 # with a desktop notification
ring = "30s"                 # how long it rings, "1s" to "30m"
sound_file = ""              # empty = katTasks's alarm-clock beeping; .wav works everywhere
default_time = "7:00"        # where the picker starts for a new alarm

[katnotes]                   # see katnotes below
command = "katnotes"         # run by add_to_note, and by open_note in the terminal
gui_command = "katnotes-gui" # run by open_note in katTasks-gui

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
| `toggle_doing` | `i` | start / stop working on it (`status:doing`); see [In progress](#in-progress) |
| `progress_up` / `progress_down` | `>` / `<` | progress +10% / −10% (`progress:40`) |
| `move_down` / `move_up` | `J` / `K` | reorder a task among tasks that tie under the current sort (any task in `file` sort) |
| `visual` | `v` | visual mode: `space` marks rows; done, delete, priority and copy then apply to all marked |
| `select_all` | `V` | mark every task shown |
| `copy_line` / `copy_body` | `yy` / `yb` | copy the line / just the words, without priority, dates, `+` `@` or tags |
| `add_to_note` | (none; `:add_to_note`) | add the task to a [katnotes](#katnotes) note, picked from a list you can search as you type |
| `undo` | `u` | undo (50 steps; a bulk change is one step) |
| `archive_done` | `A` | move completed tasks to done.txt |
| `filter` | `/` | search; see below |
| `save_search` | `fs` | save the current search under a name |
| `saved_searches` | `ff` | jump to saved searches in the sidebar |
| `clear_filter` | `esc` | leave visual mode, then clear the search, then the sidebar filter |
| `cycle_sort` | `s` | priority → due → created → file order |
| `toggle_show_done` | `H` | hide / show completed tasks |
| `calendar` | `c` | a month of due dates, in place of the list (see below) |
| `agenda` | `w` | the list as one week, by day and time (see below) |
| `toggle_line_numbers` | `L` | show task numbers |
| `theme_picker` | `T` | pick a theme, previewing as you move |
| `cycle_theme` | (none) | switch to the next theme |
| `open_note` | (none; `:open_note`) | open the task's linked note in [katnotes](#katnotes): its terminal app here, katnotes-gui from katTasks-gui |
| `palette` | `:` `ctrl+p` | fuzzy-search every action, and your saved searches |
| `reload` | `r` | re-read todo.txt from disk |
| `reload_config` | `ctrl+r` | re-read the settings |
| `settings` | (none; `:settings`) | open the settings file in your editor, then re-read it when the editor closes |
| `config` | (none; `:config`) | open the settings folder (themes, scripts) in your editor, then re-read the settings |
| `help` | `?` | all keys, what to type in a task, and the tags and search terms; `j` `k`, `pgup` `pgdn`, `g` `G` scroll it when it doesn't fit |
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
with a reminder of the `(A)`, `+project`, `@context`, `due:`, `rec:`,
`alarm:`, `at:` and `status:` / `progress:` syntax underneath. While typing a
task, `tab` (or `enter`) completes `+project`, `@context`, `due:`, `rec:` and
`at:` words; `←→` or `↑↓` choose among the suggestions.
`at:` offers the next half hours, and the times that fit what you've typed
(`at:9` → `at:900`, `at:915`…; `at:3p` → `at:3pm`). Typing `alarm:` opens a
[date and time picker](#alarms).

Typing `due:` opens a month calendar on today. `←→` moves a day, `↑↓` a
week and `[` `]` a month. `enter` writes the date, as in `due:2026-12-01`,
and `esc` closes the calendar and leaves `due:` for you to finish. Any other
key closes it and is typed into the line, so you can still type `due:fri` or
`due:+3d` without stopping. `tab` on a `due:` word already in the line opens
the calendar on that date. Relative due dates are converted to
`YYYY-MM-DD` when you save: `due:today`, `due:tomorrow`, `due:fri`,
`due:+3d`, `due:+2w`, `due:+1m`. When a project or context is selected in the
sidebar, `a` fills it in for you.

## Calendar

`c` shows a month calendar in place of the task list. Each day shows a dot
and the number of open tasks due that day, in the overdue color for past
days, and a `✓` when everything due that day is done. The chosen day's tasks
are listed under the month. The calendar shows the tasks of the sidebar entry
and search, so with `+work` selected you see only work.

| keys | |
|---|---|
| `h` `l`, arrows | previous / next day |
| `j` `k`, arrows | next / previous week |
| `[` `]`, `pgup` `pgdown` | previous / next month |
| `t`, `g` | today |
| `enter` | list the day's tasks (a `due:` search), where `x`, `e`, `d`… work on them; `esc` clears it |
| `a` | add a task due that day |
| `esc`, `c` | close the calendar |

The keys follow your `[keys]` settings: whatever you bound to `down` moves a
week, to `focus_sidebar` a day back, and so on. `tab` still goes to the
sidebar, and `/`, `s`, `H`, `:` and `?` work as in the list. Weeks start on
Sunday.

## Agenda

`w` shows one week of the list (Sunday to Saturday), grouped by due day.
Under each day come its tasks without a time, then its timed tasks in time
order with the time in the margin. It's still the task list: `j` `k` move
over the week's tasks and `x`, `e`, `d`, `p`, `yy` and visual mode work on
them. It follows the sidebar entry and search, like the list.

| keys | |
|---|---|
| `h` `l`, arrows, `[` `]` | previous / next week |
| `t` | this week |
| `a` | add a task due on the day under the cursor |
| `c` | the calendar, on the same day (`w` there comes back) |
| `esc`, `w` | close the agenda |

A task's time is an `at:` tag (`at:1430`, `at:930`, `at:9`, `at:3pm`,
`at:915am`, and `at:14:30` as earlier versions wrote it) or a time in its
text, such as `Standup 9:30`, `Standup 9.30` or `Call mom 3pm`. In text a
time needs a colon, a dot or am/pm, so a plain number like `4` or `2026`
isn't taken for one. `due:` stays a plain date, so other todo.txt tools still
read the file.

In katTasks-gui the agenda is seven columns: each day's untimed tasks at the
top, then its timed tasks placed down the day by time.

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
| `status:doing`, `status:any`, `status:none` | by `status:` tag |
| `progress:40`, `progress:>50`, `progress:<50`, `progress:25-75`, `progress:any`, `progress:none` | by `progress:` percent |
| `-term` | negates any term, e.g. `-@waiting`, `-due:none` |

Matching words are highlighted. `fs` saves the current search (or the selected
project or context) to `[filters]`. Saved searches appear in the sidebar under
SAVED and in the palette.

## Alerts

A task with an `at:` tag alerts you at that time, with a desktop notification
and a sound: `Dentist due:2026-12-01 at:1430` on December 1, and `Standup
at:930` (no `due:`) every day. Only `at:` tags count, not times written in
the text. Done tasks don't alert. Set it up under `[alerts]` in the settings:

```toml
[alerts]
enabled = true
lead = "10m"       # alert this long before the at: time; "0m" at the time
popup = true       # desktop notification
sound = true
sound_file = ""    # e.g. "~/sounds/bell.wav"; empty = the system's notification sound
```

From the command line, `katTasks alert +5m Check the oven` adds a task that
alerts in 5 minutes (it sets `due:` to that day and the `at:` time `lead`
later), `katTasks alert 3 930` gives task 3 a time, and `katTasks alerts` lists
what's coming up in the next week.

katTasks and katTasks-gui alert while they're open. For alerts when neither
is, run the background reminder at login:

    katTasks remind -install     # start it at login (-uninstall stops that)
    katTasks remind -test        # send a sample alert now, to check your setup

Each alert goes off once, whichever of them is running. An alert missed
while the computer slept still goes off within 15 minutes of its time;
later than that it's skipped.

| OS | notification | sound | started at login by |
|---|---|---|---|
| Linux | `notify-send` (libnotify), or `gdbus` | `canberra-gtk-play`, or `paplay` / `pw-play` / `aplay` | `~/.config/autostart/katTasks-remind.desktop` |
| macOS | Notification Center, through `osascript` | the "Glass" sound; `afplay` for `sound_file` | a launch agent in `~/Library/LaunchAgents` |
| Windows | a toast, through Windows PowerShell | the toast's sound; `sound_file` must be `.wav` | the `katTasks remind` Run entry for your user |
| WSL | a Windows toast, as on Windows | as on Windows | install it on the Windows side (below) |

The Windows installer sets up and starts the reminder itself, and the
uninstaller removes it. Under WSL, `task install` doesn't; run `katTasks.exe
remind -install` from Windows once if you want it. On macOS the
notifications come from Script Editor, which macOS may ask you to allow the
first time.

## Alarms

An `alarm:` tag rings like an alarm clock at a date and time, for when a
quiet notification isn't enough: `Leave for airport
alarm:2026-12-01T0530`. Typing `alarm:` while adding or editing a task
opens a picker:

| keys | |
|---|---|
| `←` `→` `↑` `↓` (or `h` `l` `k` `j`) | day, week |
| `[` `]` | month |
| `t` / `T` | today / tomorrow |
| `tab` | switch between the date and the time |
| `↑` `↓`, `←` `→` in the time | 15 minutes, an hour |
| type `6:45`, `645` or `7pm` | set the time |
| `enter` / `esc` | set it / cancel |

`tab` on an `alarm:` already in the line opens the picker on it again. In
katTasks-gui you can also click a day (double-click sets it) and the time's
arrows. A new alarm starts at `alarms.default_time`, today if that's still
ahead and tomorrow if not. The list shows it as `⏰ tomorrow 5:30`.

It rings with a built-in beep-beep-beep-beep for `alarms.ring` (30 seconds
by default), or with your own `sound_file`, and with an urgent notification
if `popup` is on. Alarms are sent like [alerts](#alerts): by katTasks,
katTasks-gui and `katTasks remind` in the background, once each, and up to
15 minutes late after sleep. Done tasks don't ring. When a `rec:` task is
completed, its next copy's alarm moves along with its due date.
`katTasks remind -test-alarm` rings a sample one now.

From the command line: `katTasks alarm +5m Tea is ready` adds a task that
rings in 5 minutes, and `katTasks alarm 3 tomorrow 6:30` (or `fri 7am`,
`2026-12-01 0730`, `in 2h`, `18:00`) gives task 3 an alarm. In `+5m`, `m` is
minutes, not months as in `due:+1m`; `+3d` and `+2w` are days and weeks.

## In progress

todo.txt itself only has done and not done, so katTasks keeps the steps in
between in two ordinary tags, which other todo.txt tools keep and ignore:

| tag | meaning | shown as |
|---|---|---|
| `status:doing` | being worked on | `▶ doing` |
| `status:blocked`, `status:review`, … | any other state, typed by hand | the word |
| `progress:40` | 40% done (0–100; `progress:40%` also works) | `▰▰▱▱▱ 40%` |

`i` adds or removes `status:doing`, and `>` / `<` move `progress:` up or down
by 10% (down to 0 removes it). They work on every marked task in visual mode,
and are one undo step. Reaching 100% doesn't complete a task; `x` does. A done
task keeps both tags, so reopening it restores them, but the next copy of a
`rec:` task starts without them. Search with `status:doing` or
`progress:>50`, or save one as a sidebar search:

```toml
[filters]
doing = "status:doing"
```

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

## Lua scripts

See [SCRIPTING.md](SCRIPTING.md) for a full guide with examples.

Put `.lua` files in a `scripts/` folder next to config.toml (on Linux
`~/.config/katTasks/scripts/`). A `-- comment` on the first line becomes the
script's description. Each script `NAME.lua` can be run two ways:

- **From `:`** as `script:NAME`. It works on the task under the cursor, and a
  returned string is shown in the footer.
- **While typing a task**, as a `/NAME` word. Typing `/` suggests up to five
  matching scripts; `tab` runs the highlighted one, and any `/NAME` still in
  the text runs when you press `enter`. What the script returns replaces the
  word, so `return nil` removes it. A `/word` that isn't a script is kept.

Scripts get a `kat` table:

| | |
|---|---|
| `kat.input` | the task being typed, without the `/NAME` word (`""` from `:`) |
| `kat.task` | the task under the cursor (or being edited), or `nil` |
| `kat.tasks()` | every task in todo.txt |
| `kat.today` | today as `YYYY-MM-DD` |
| `kat.date(s)` | `"fri"`, `"+3d"`, `"tomorrow"`… as `YYYY-MM-DD`, or `nil` |
| `kat.add(line)` | add a task |
| `kat.update(line)` | replace the task under the cursor (from `:` only) |
| `kat.choose(prompt, {…})` | show a list to pick from; returns the pick, or `nil` on `esc` (`/NAME` only) |
| `kat.replace(line)` | replace the whole line being typed (`/NAME` only) |
| `kat.status(msg)` | show a message in the footer; `print` does the same |

A task is a table with `line`, `text`, `done`, `priority`, `due`, `projects`,
`contexts` and `tags` (`tags.id` for `id:7`). Everything a script adds or
changes is one undo step, and a script is stopped after 2 seconds. Scripts are
read again each time you open `:` or start typing a task, so new ones show up
without a restart.

```lua
-- due a week from today
return "due:" .. kat.date("+1w")
```

```lua
-- copy this task to next week
local t = kat.task
if not t then return "no task" end
kat.add(t.text:gsub("due:%S+", "") .. " due:" .. kat.date("+1w"))
```

## katnotes

[katnotes](https://github.com/devinthemtn/katnotes) (Markdown notes) can link
a task in a note to a line here, so finishing it in either app finishes it in
both. With `enabled = true` under `[todo_txt]` in katnotes' settings, a linked
line carries two tags:

    (A) 2026-11-23 call the plumber due:2026-11-27 kn:7f3a2c note:page/house

- `kn:` (the link's ID) and `note:` are drawn in the theme's `link` color,
  and a linked task shows `⇄` beside its due date.
- **Linked to notes** in the sidebar lists the linked tasks.
- `:add_to_note` adds the task under the cursor to a note: pick one (or
  today's journal) from the list, typing to narrow it. katnotes writes the
  task into the note and the tags here, and the list reloads.
- `:open_note` opens the linked note: the katnotes terminal app in place of
  katTasks (quit it to come back), or katnotes-gui from katTasks-gui, which
  switches an open katnotes-gui window to the note.
- Both run the programs set under `[katnotes]` in the settings (`katnotes`
  and `katnotes-gui` on your `$PATH` by default).

Don't edit or remove the `kn:` and `note:` tags by hand. katnotes keeps the
text, due date, priority and done state in step from the note.

## Themes

Built in: `default`, which adapts to light and dark terminals; `terminal`,
which uses only your terminal's 16 colors; `nord`; `gruvbox`; and
[`dracula`](https://draculatheme.com). Press `T` to pick one. The choice is saved to `theme` in config.toml.

Any `themes/NAME.toml` next to config.toml adds a theme called `NAME`. If it
has the same name as a built-in, it replaces it. `katTasks -make-theme NAME`
writes the current theme there with every color explained. A theme only needs
the colors it changes; the rest come from `base`:

```toml
base = "nord"
accent = "#ff79c6"
cursor_bg = { light = "#eeeeee", dark = "#333344" }   # differs on light / dark terminals
```

A color is `"#rrggbb"`, an ANSI number from `"0"` to `"255"`, or `""` for the
terminal's own color. The colors are `fg dim accent border border_focus
cursor_bg selected_bg match_bg title_fg title_bg project context tag highlight
pri_a pri_b pri_c pri_d pri_other due today overdue done ok error`. A theme file with
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
| `start N…` / `stop N…` | set / clear `status:doing` |
| `status N WORD` | any status, e.g. `blocked`; `none` clears it |
| `progress N PERCENT` (`prog`) | `40`, `+10`, `-10`; `none` clears it |
| `alert N TIME` (`at`) | time of day for the agenda and alerts: `930`, `14:30`, `3pm`; `+5m` or `in 2h` alerts that long from now, once; `none` clears it |
| `alarm N [DATE] TIME` | an alarm: `+5m`, `in 1h30m`, `tomorrow 6:30`, `fri 7am`, `2026-12-01 0730`, or just `18:00` (the next one); `none` clears it |
| `alert WHEN TEXT…`, `alarm WHEN TEXT…` | add a task with that alert or alarm: `katTasks alarm +5m Tea is ready` |
| `alerts [DAYS]` (`upcoming`) | the alerts and alarms of the next DAYS days (7), soonest first |
| `remind` | run the background reminder for alerts and alarms; `-test`, `-test-alarm`, `-install`, `-uninstall`, see [Alerts](#alerts) |

```
$ katTasks alarm +5m Tea is ready
$ katTasks alert in 20m Move the laundry
$ katTasks add "Pay rent +Home due:2026-12-01 rec:+1m"
4 2026-11-18 Pay rent +Home due:2026-12-01 rec:+1m
TODO: 4 added.
$ katTasks ls +Home due:+1m
$ katTasks -json ls pri:A | jq '.[].text'
$ katTasks -f del 4
```

With `-json`, lists print an array of tasks
(`n line done priority created completed text projects contexts tags due at
alarm status progress`); `alerts` prints `{kind, at, fires, task}`.
Changes print `{action, tasks, added, message}`.

## Your todo.txt

katTasks keeps the file plain todo.txt that any other todo.txt app can read
and edit. Everything it adds is an ordinary `key:value` tag, which other
apps keep and ignore:

| tag | meaning |
|---|---|
| `due:2026-12-01` | due date (relative dates you type become this) |
| `t:2026-11-28` | threshold date, moved along by `rec:` |
| `rec:1w`, `rec:+1m` | repeat on completion |
| `at:1430`, `at:3pm` | time of day: agenda and alerts |
| `alarm:2026-12-01T0730` | alarm date and time |
| `status:doing`, `progress:40` | in progress |
| `pri:A` | the priority of a completed task, so reopening restores it |

The format doesn't allow a colon in a tag's value, so times are written
`at:1430` and `alarm:…T0730`. Files written by earlier versions, with
`at:14:30`, still read the same; a task's tags are rewritten in the new form
only when you add or edit that task.

Saving changes only the lines of tasks you changed: every other line is
written back exactly as it was, spacing included. Windows line endings
(`\r\n`) and a byte order mark at the start stay as the file had them.
Blank lines are dropped.

## Safety

- Every save writes a temporary file, syncs it to disk and renames it over todo.txt, so a crash or power cut can't leave the file half-written. Its permissions are kept, and if todo.txt is a symlink, the file it points to is saved and the link stays.
- If another program changes todo.txt while katTasks is open, edits are refused until you press `r` to reload. katTasks never overwrites those changes.
- Archiving writes done.txt before removing anything from todo.txt. If todo.txt can't be saved, done.txt is put back as it was, so the tasks are never in both files.

## Releasing

Releases are built by [GoReleaser](https://goreleaser.com) in GitHub Actions
(`.github/workflows/release.yml`, configured by `.goreleaser.yaml`). Pushing a
tag runs the tests, builds every platform, and publishes the release with
archives, checksums and a changelog from the commits. The desktop app is built
first, on Linux (x86-64 and ARM64), Windows (x86-64 and ARM64) and macOS (one
universal `.app`), and its `katTasks-gui_…` archives go in the same release,
with the Windows installer (`katTasks-<version>-windows-amd64-setup.exe`):

    task bump-version V=1.2.0   # the version in the GUI's build files; commit it
    git tag -a v1.2.0 -m "katTasks 1.2.0"
    git push origin v1.2.0

The tag is the version: `katTasks -version` and `katTasks-gui -version` report
it. The GUI's build files keep their own copy, which Windows file properties,
macOS About and the Linux packages show, so `task bump-version` sets it in
each of them (and in `frontend/package.json`). Edit the generated notes on
GitHub afterwards if you like. To try the build without publishing, run the
workflow by hand (Actions → release → Run workflow); the archives are attached
to the run.

`task release`, run on the tagged commit, builds the Linux x86-64 and Windows
files locally into `bin/release-<version>/`, with the names the workflow gives
them: `katTasks_<version>_linux_amd64.tar.gz`,
`katTasks-gui_<version>_linux_amd64.tar.gz`, the Windows installer and
`checksums.txt`. It stops if the working tree isn't clean, the GUI's build
files aren't at the tag's version or CHANGELOG.md has no section for it.
Every platform locally:
`go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean`.

## License

MIT; see [LICENSE](LICENSE).
