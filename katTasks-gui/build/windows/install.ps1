# Sets up an installed katTasks for the current Windows user (no admin):
# a Start Menu shortcut for katTasks-gui.exe, the install folder on the user
# PATH (so katTasks.exe works in PowerShell and cmd), and optionally the WSL
# todo.txt as the Windows files.todo. Run by `task install-windows` from WSL.
param(
    [Parameter(Mandatory)][string]$Dir,
    # The WSL todo.txt (\\wsl.localhost\<distro>\...). It becomes files.todo
    # only while the Windows settings still use the default todo.txt and that
    # file has no tasks, so a choice made on Windows is never overwritten.
    [string]$TodoFile
)

$ErrorActionPreference = 'Stop'

# One "  Label         text" line, in the same columns as the Taskfile's
# summary. ASCII only: WSL may not decode other characters from PowerShell.
function Say([string]$Label, [string]$Text) { Write-Host ('  {0,-17} {1}' -f $Label, $Text) }

$gui = Join-Path $Dir 'katTasks-gui.exe'
$cli = Join-Path $Dir 'katTasks.exe'

$lnk = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\katTasks.lnk'
$s = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
$s.TargetPath = $gui
$s.WorkingDirectory = $env:USERPROFILE
$s.IconLocation = "$gui,0"
$s.Description = 'Keyboard-driven todo.txt task manager'
$s.Save()
Say 'Start Menu' 'katTasks shortcut, opens katTasks-gui.exe'

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @($path -split ';' | Where-Object { $_ })
if ($parts -notcontains $Dir) {
    [Environment]::SetEnvironmentVariable('Path', (($parts + $Dir) -join ';'), 'User')
    Say 'PATH' 'added the folder above (new terminals pick it up)'
} else {
    Say 'PATH' 'already has the folder above'
}

if ($TodoFile) {
    # katTasks -paths prints "settings: …", "todo: …" and "done: …", and
    # writes the default settings file if there is none yet.
    $paths = @{}
    foreach ($line in (& $cli -paths)) {
        $k, $v = $line -split ': ', 2
        $paths[$k] = $v
    }
    $settings, $current = $paths['settings'], $paths['todo']
    $default = Join-Path $env:USERPROFILE 'todo.txt'
    $pattern = '(?m)^todo\s*=\s*"~/todo\.txt"\s*$'
    if ($current -eq $TodoFile) {
        Say 'Tasks' "$TodoFile, shared with WSL"
    } elseif ($current -ne $default) {
        Say 'Tasks' "$current (files.todo in the Windows settings; left as it is)"
    } elseif ((Test-Path $default) -and (Get-Content $default -Raw -ErrorAction SilentlyContinue) -match '\S') {
        Say 'Tasks' "$default, which already has tasks, so it stays"
        Say '' "to share the WSL tasks instead, set files.todo in $settings"
    } else {
        $text = Get-Content $settings -Raw
        if ($text -notmatch $pattern) {
            Say 'Tasks' "couldn't find the default files.todo line in $settings"
            Say '' "set it to '$TodoFile' to share the WSL tasks"
        } else {
            # A TOML literal string ('…'), so the backslashes need no escaping.
            # $ would start a group reference in the replacement.
            $text = [regex]::Replace($text, $pattern, "todo = '$($TodoFile.Replace('$', '$$'))'")
            [IO.File]::WriteAllText($settings, $text, (New-Object Text.UTF8Encoding $false))
            Say 'Tasks' "$TodoFile, now shared with WSL"
        }
    }
}
