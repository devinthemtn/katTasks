# Changelog

## 1.0.0 (unreleased)

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
