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
$gui = Join-Path $Dir 'katTasks-gui.exe'
$cli = Join-Path $Dir 'katTasks.exe'

$lnk = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\katTasks.lnk'
$s = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)
$s.TargetPath = $gui
$s.WorkingDirectory = $env:USERPROFILE
$s.IconLocation = "$gui,0"
$s.Description = 'Keyboard-driven todo.txt task manager'
$s.Save()
Write-Host "Start Menu: katTasks -> $gui"

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @($path -split ';' | Where-Object { $_ })
if ($parts -notcontains $Dir) {
    [Environment]::SetEnvironmentVariable('Path', (($parts + $Dir) -join ';'), 'User')
    Write-Host "Added $Dir to your PATH (new terminals pick it up)"
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
        Write-Host "Tasks: $TodoFile"
    } elseif ($current -ne $default) {
        Write-Host "Tasks: keeping files.todo = $current (set in the Windows settings)"
    } elseif ((Test-Path $default) -and (Get-Content $default -Raw -ErrorAction SilentlyContinue) -match '\S') {
        Write-Host "Tasks: $default already has tasks, so it stays; set files.todo in $settings to use the WSL one"
    } else {
        $text = Get-Content $settings -Raw
        if ($text -notmatch $pattern) {
            Write-Host "Tasks: couldn't find the default files.todo line in $settings; set it to '$TodoFile' to share the WSL tasks"
        } else {
            # A TOML literal string ('…'), so the backslashes need no escaping.
            # $ would start a group reference in the replacement.
            $text = [regex]::Replace($text, $pattern, "todo = '$($TodoFile.Replace('$', '$$'))'")
            [IO.File]::WriteAllText($settings, $text, (New-Object Text.UTF8Encoding $false))
            Write-Host "Tasks: $TodoFile (shared with WSL)"
        }
    }
}
