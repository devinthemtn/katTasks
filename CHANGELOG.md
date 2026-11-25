# Changelog

## 2.0.3 (2026-11-25)

- `-v` works as `-version` does, in `katTasks` and `katTasks-gui`.
- New `files.watch` setting: while open, katTasks and katTasks-gui recheck
  todo.txt this often (default `"3m"`) and reload it if another program
  changed it, keeping the cursor on its task. `false` turns it off. A change
  found while you're typing a task or have tasks selected waits for the next
  check. `katTasks -update-config` adds the setting to an existing file.
- `:sync` does the same check right away.
- In the add/edit popup, `←` `→` always move the cursor; they no longer get
  stuck on `+project`, `@context`, `due:` and similar words. `↑` `↓` choose a
  suggestion, and the list shows only while you type, so moving the cursor
  and pressing `enter` saves instead of completing.
- The add/edit box underlines the words it has a list or picker for
  (`+project`, `@context`, `due:`, `rec:`, `alarm:`, `at:`). `tab` anywhere
  on one lists what it could be instead and replaces the whole word.
- A long task wraps onto more lines of the add/edit box instead of
  scrolling sideways, in katTasks and katTasks-gui.

## 2.0.1 (2026-11-23)

- `task release` builds the Linux x86-64 and Windows release files locally
  into `bin/release-<version>/`, with checksums. It stops if the tree isn't
  clean, the GUI's build files aren't at the tag's version or this file has
  no section for it.
- `task bump-version V=x.y.z` sets the version the GUI's build files carry
  (Windows file properties, macOS About, Linux packages), which had been
  left behind.
- The README covers the katnotes integration and the release steps.

## 2.0.0 (2026-11-23)

Tasks can be linked to notes in
[katnotes](https://github.com/devinthemtn/katnotes), so finishing one in
either app finishes it in both. See "katnotes" in the README.

- A linked task carries `kn:<id>` and `note:<kind/id>` tags, drawn in the
  theme's new `link` color, and shows `⇄` beside its due date. Theme files
  that don't set `link` get it from their `base`.
- **Linked to notes** in the sidebar lists the linked tasks.
- `add_to_note` (`:add_to_note`) adds the task to a katnotes note, picked
  from a list you can search as you type.
- `open_note` (`:open_note`) opens the linked note: the katnotes terminal app
  from the terminal, katnotes-gui from katTasks-gui.
- New `[katnotes]` settings, `command` and `gui_command`, name the programs
  to run: a bare name is looked up on `$PATH`, or give a full path.

### Changed

- Completing a recurring (`rec:`) task that is linked leaves `kn:` and
  `note:` on the finished task, which is the one the note stands for. The
  next occurrence starts unlinked. Unlinked tasks recur as before, and
  nothing else in the 1.0 promise changed.

## 1.1.0 (2026-11-22)

- Text inside `{{ }}` in a task is drawn in the theme's new `highlight`
  color, in the terminal UI and the GUI. `{{` and `}}` may be several words
  apart. Theme files that don't set it get it from their `base`.

## 1.0.0 (2026-11-21)

### What 1.0 promises

From 1.0 on, these stay compatible until a 2.0. New things may be added
(commands, flags, settings, JSON fields, Lua functions), but nothing below
is renamed, removed or changed in meaning:

- **Commands and flags:** every command and alias in `katTasks -h`, and every
  flag it lists.
- **`-json` output:** the fields of tasks, of `alerts` and of change results.
  Fields may be added; scripts should ignore ones they don't know. The tests
  in `internal/cli/testdata/json` pin the format.
- **Settings:** every key in `config.toml`, as `katTasks -make-config` writes it.
- **Lua:** the `kat` table and task tables described in [SCRIPTING.md](SCRIPTING.md).
- **todo.txt tags:** what katTasks reads and writes (`due:`, `rec:`, `at:`,
  `alarm:`, `status:`, `progress:`, `pri:` on done tasks) keeps its meaning,
  so a todo.txt from 1.0 opens the same in every later 1.x.

### Changes that can break scripts

- **`-json`: a task's `raw` field is now `line`**, the name Lua already uses.
  Change `.raw` to `.line` in scripts that read it.
- **`-json`: `katTasks alert` reports `"action": "alert"`** instead of `"at"`.
  `alert` is now the command's main name; `at` still works as an alias.
- **Setup flags are hyphenated:** `-make-config`, `-update-config` and
  `-make-theme`. The old `-makeconfig`, `-updateconfig` and `-maketheme`
  still work, but help no longer lists them.

### New

- `:settings` opens the settings file in your editor, and `:config` opens
  the settings folder (themes, scripts); both re-read the settings when the
  editor closes. In the terminal and the desktop app.
- `task install` adds lowercase `kattasks` and `kattasks-gui` links next to
  the apps (skipped on case-insensitive filesystems, where they'd be the
  same file).
- katTasks is MIT-licensed; `LICENSE` is in every archive.

### Fixes

- **Saving is safe on a crash or power cut:** todo.txt and `config.toml` are
  synced to disk before they replace the old file, so a power cut can't leave
  an empty file. A symlinked todo.txt or `config.toml` stays a symlink.
- **Archiving can't duplicate tasks:** if todo.txt can't be saved, done.txt is
  put back as it was. Before, archiving again copied the tasks to done.txt
  twice. With `auto_archive`, a failed archive no longer shows tasks as done.
- **Two hangs:** a huge business-day recurrence (`rec:100000000b`) or
  `due:-Nb` froze the app, which counted the days one by one.
- A task with a header and no text (`x `, `(Z) `) now saves back unchanged,
  and a first line holding only a byte order mark no longer loads as an
  empty task.
- On Windows, opening the editor no longer gets a broken path.
- SCRIPTING.md said a task's `projects` and `contexts` come without their `+`
  and `@`. They come with them, and the "pick from projects" example no
  longer offers `++family`.

### Other

- CI runs the tests on Linux, macOS and Windows, on the newest Go and on the
  oldest supported one (1.24.2), plus `govulncheck` on both apps.
- Release builds use Go 1.26.11, with its security fixes. `go install` still
  works on Go 1.24 or newer.
- Many new tests and fuzz targets; the large ui and cli files are split up,
  and the terminal and desktop apps share one footer.
