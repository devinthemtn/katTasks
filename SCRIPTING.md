# Scripting katTasks with Lua

katTasks runs small [Lua](https://www.lua.org/manual/5.1/) scripts that you
write. You can use them to add tags for you, change the task you're typing,
ask you to pick from a list, or add several tasks in one go. This guide covers
everything a script can do, with examples you can copy.

- [Where scripts live](#where-scripts-live)
- [Running a script](#running-a-script)
- [Your first script](#your-first-script)
- [The `kat` table](#the-kat-table)
- [Tasks as Lua tables](#tasks-as-lua-tables)
- [What a script's return value does](#what-a-scripts-return-value-does)
- [Asking the user: `kat.choose`](#asking-the-user-katchoose)
- [Rewriting the whole line: `kat.replace`](#rewriting-the-whole-line-katreplace)
- [Examples](#examples)
- [Limits and gotchas](#limits-and-gotchas)
- [Debugging](#debugging)

## Where scripts live

Scripts are `.lua` files in a `scripts/` folder next to your `config.toml`:

| OS | Folder |
|---|---|
| Linux | `~/.config/katTasks/scripts/` |
| macOS | `~/Library/Application Support/katTasks/scripts/` |
| Windows | `%AppData%\katTasks\scripts\` |

(The folder is next to whichever `config.toml` katTasks loads, so if you point
katTasks at a different config file, its scripts come from there.)

The file name, minus `.lua`, is the script's name: `pri.lua` is the `pri`
script. Names can't contain spaces.

If the first line is a `--` comment, it's used as the script's description
in the `/` suggestions and in the `:` palette:

```lua
-- pick a priority (A–C) for this task
```

katTasks reads the folder again each time you open `:` or start typing a task,
so you don't have to restart it after adding or changing a script.

## Running a script

There are two ways to run a script, and they see slightly different things.

**Inline, as `/name`, while adding or editing a task.** Type `/` and up to five
matching scripts are suggested. `tab` runs the highlighted one right away. Any
`/name` still in the text when you press `enter` runs before the task is
saved. A `/word` that isn't the name of a script stays as plain text, so
`and/or` is fine.

**From the `:` palette, as `script:name`.** This works on the task under the
cursor. Use it for scripts that act on an existing task or add new ones.

| | inline `/name` | palette `script:name` |
|---|---|---|
| `kat.input` | the line being typed, without the `/name` word | `""` |
| `kat.task` | the task being edited (`nil` when adding) | the task under the cursor |
| return value | replaces the `/name` word | shown in the footer |
| `kat.update` | not allowed | allowed |
| `kat.choose`, `kat.replace` | allowed | not allowed |

## Your first script

Save this as `scripts/week.lua`:

```lua
-- due a week from today
return "due:" .. kat.date("+1w")
```

Now add a task, type `Pay rent /wee`, and press `tab`. The `/week` word turns
into something like `due:2026-11-27`.

## The `kat` table

Every script gets a global table called `kat`.

| Field | What it is |
|---|---|
| `kat.input` | Inline: the line being typed, without the `/name` word. From `:`: `""`. |
| `kat.task` | The task under the cursor or being edited, as a [task table](#tasks-as-lua-tables), or `nil`. |
| `kat.tasks()` | Every task in todo.txt, as a list of task tables. |
| `kat.today` | Today, as `YYYY-MM-DD`. |
| `kat.date(s)` | Turns `"fri"`, `"+3d"`, `"+2w"`, `"tomorrow"` or `"2026-12-01"` into `YYYY-MM-DD`, or `nil` if it can't. |
| `kat.add(line)` | Adds a new task. Call it as often as you like. |
| `kat.update(line)` | Replaces the task under the cursor. Only from `:`. |
| `kat.choose(prompt, options)` | Shows a list and returns the option the user picked, or `nil` if they pressed `esc`. Only inline. See [below](#asking-the-user-katchoose). |
| `kat.replace(line)` | Replaces the whole line being typed. Only inline. See [below](#rewriting-the-whole-line-katreplace). |
| `kat.status(msg)` | Shows a message in the footer. `print(...)` does the same. |

## Tasks as Lua tables

`kat.task` and the items of `kat.tasks()` look like this:

| Field | Example |
|---|---|
| `line` | `"(A) 2026-11-20 Call mom +family due:2026-11-22"`: the full todo.txt line |
| `text` | `"Call mom +family due:2026-11-22"`: the line without the done mark, priority or dates |
| `done` | `false` |
| `priority` | `"A"`, or `nil` |
| `due` | `"2026-11-22"`, or `nil` |
| `projects` | `{ "family" }` |
| `contexts` | `{ "phone" }` |
| `tags` | `{ due = "2026-11-22", id = "7" }`: every `key:value` |

These tables are copies. Changing them changes nothing in todo.txt. To
change things, use `kat.add`, `kat.update` or `kat.replace`.

## What a script's return value does

- **Inline:** the returned value takes the place of the `/name` word.
  Return a string to insert text, or `nil` (or nothing at all) to just delete
  the word.
- **From `:`:** a returned string is shown in the footer, unless the script
  already set a message with `kat.status`.

Everything one script adds or changes is a single undo step, so `u` undoes
all of it at once.

## Asking the user: `kat.choose`

```lua
local pick = kat.choose("Priority", { "A", "B", "C" })
```

This opens a list over the task popup. The user can pick in any of these
ways:

- `↑` `↓` (or `j` `k`) to move, then `enter`
- a digit, `1` to `9`, for that row
- the first letter of an option, when only one option starts with that letter

`esc` cancels, and `kat.choose` returns `nil`. Always handle that case:

```lua
local pick = kat.choose("Priority", { "A", "B", "C" })
if not pick then return nil end   -- cancelled: just remove the /name word
```

**How it works, and why it matters.** A script can't pause halfway. When it
reaches a `kat.choose` it has no answer for yet, katTasks stops the script,
throws away everything it did up to that point, and shows the list. Once the
user picks, katTasks runs the script again from the top, and this time
`kat.choose` returns the pick right away. If you call `kat.choose` several
times, each pick is remembered in order, so the user just sees one list after
another:

```lua
-- schedule: pick a day, then a context
local day = kat.choose("When?", { "today", "tomorrow", "fri", "+1w" })
if not day then return nil end
local ctx = kat.choose("Where?", { "@home", "@work", "@errands" })
if not ctx then return "due:" .. kat.date(day) end
return "due:" .. kat.date(day) .. " " .. ctx
```

This has a few consequences:

- Put your `kat.choose` calls **before** you call `kat.add`, `kat.status` or
  anything else with an effect. Anything the script did before the last
  question is thrown away and done again on the next run, so it isn't doubled,
  but it's clearer to ask first.
- Ask the same questions in the same order every time the script runs, so
  each answer lines up with the right question. Choosing *which* question to
  ask next based on an earlier answer is fine.
- Don't wrap `kat.choose` in `pcall`.

If the user pressed `enter` to save and a `/name` script opens a list, the
task is saved as soon as they pick. If they cancel, the popup stays open.

## Rewriting the whole line: `kat.replace`

The return value can only replace the `/name` word. Some things have to go
somewhere else in the line. A priority, for example, has to come first. In
that case, call `kat.replace(line)` with the whole new line. `kat.input` is
the current line without your `/name` word, so it's a good starting point:

```lua
kat.replace("(A) " .. kat.input)
```

If the script calls `kat.replace` and also returns a string, the string goes
on the end of the new line.

## Examples

### Pick a priority (`pri.lua`)

```lua
-- pick a priority (A–C) for this task
local pick = kat.choose("Priority", { "A", "B", "C" })
if not pick then return nil end -- esc: just drop the /pri word

-- Priority must lead the line (after "x " on a done task), so rewrite the
-- whole line, replacing any priority already there.
local line = kat.input
local done = ""
if line:sub(1, 2) == "x " then done, line = "x ", line:sub(3) end
line = line:gsub("^%([A-Z]%)%s*", "")
kat.replace(done .. "(" .. pick .. ") " .. line)
```

Type `Call mom /pri`, press `tab`, then `b`. The line becomes `(B) Call mom`.

### Add a project from a list

```lua
-- tag with one of my projects
local p = kat.choose("Project", { "+home", "+work", "+garden" })
return p   -- nil just removes the /word
```

### Pick from projects you already use

```lua
-- tag with a project already in todo.txt
local seen, list = {}, {}
for _, t in ipairs(kat.tasks()) do
  for _, p in ipairs(t.projects) do
    if not seen[p] then seen[p] = true; table.insert(list, "+" .. p) end
  end
end
table.sort(list)
if #list == 0 then kat.status("no projects yet"); return nil end
return kat.choose("Project", list)
```

### Copy a task to next week (from `:`)

```lua
-- copy this task to next week
local t = kat.task
if not t then return "no task" end
kat.add(t.text:gsub("due:%S+", "") .. " due:" .. kat.date("+1w"))
return "copied to next week"
```

### Count what's due today (from `:`)

```lua
-- how many tasks are due today
local n = 0
for _, t in ipairs(kat.tasks()) do
  if not t.done and t.due == kat.today then n = n + 1 end
end
return n .. " due today"
```

## Limits and gotchas

- Scripts run in [gopher-lua](https://github.com/yuin/gopher-lua), which is
  Lua 5.1. There's no `goto` and no integer division with `//`.
- A script is stopped after **2 seconds**.
- `print` writes to the footer, not to the terminal.
- `string.gsub` returns two values, the new string and how many matches it
  replaced. `local s = x:gsub(...)` keeps only the string, but
  `f(x:gsub(...))` passes both to `f`, which can be a surprise. Assign the
  result to a variable first.
- In Lua patterns, `(`, `)`, `.`, `%`, `+`, `-`, `*`, `?`, `[` and `^` are
  special. Escape them with `%`, as in `"%(A%)"`.
- An inline script's return value is put on one line. Newlines become spaces.
- A script can return another `/name`, and that one runs too, up to 32 in a
  row.

## Debugging

If a script fails, its error is shown in the footer, like
`pri: pri.lua:3: attempt to concatenate a nil value`. The task popup stays
open, so you don't lose what you typed. Use `kat.status(...)` or `print(...)`
to show values while you work on a script, and run it from `:` to try it
against the task under the cursor.
