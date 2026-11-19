package alert

import (
	"encoding/base64"
	"os"
	"unicode/utf16"
)

// psScript shows a Windows toast and plays a sound. It takes its text from
// environment variables, so nothing typed in a task is ever parsed as
// PowerShell. The toast is sent as Windows PowerShell's own app id, which
// is registered on every Windows 10 and 11, so katTasks needs no install
// step for it.
const psScript = `
$ErrorActionPreference = 'Stop'
$sound = $env:KATTASKS_SOUND
if ($env:KATTASKS_POPUP -eq '1') {
  $t = [Security.SecurityElement]::Escape($env:KATTASKS_TITLE)
  $b = [Security.SecurityElement]::Escape($env:KATTASKS_BODY)
  $audio = '<audio silent="true"/>'
  if ($sound -eq 'default') { $audio = '<audio src="ms-winsoundevent:Notification.Reminder"/>' }
  $null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
  $null = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime]
  $x = New-Object Windows.Data.Xml.Dom.XmlDocument
  $x.LoadXml("<toast><visual><binding template=""ToastGeneric""><text>$t</text><text>$b</text></binding></visual>$audio</toast>")
  $app = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
  [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show([Windows.UI.Notifications.ToastNotification]::new($x))
}
if ($sound -eq 'file') {
  (New-Object System.Media.SoundPlayer $env:KATTASKS_SOUNDFILE).PlaySync()
} elseif ($sound -eq 'default' -and $env:KATTASKS_POPUP -ne '1') {
  [System.Media.SystemSounds]::Asterisk.Play()
  Start-Sleep -Milliseconds 1500
}
`

// encoded is a script as powershell.exe -EncodedCommand wants it: UTF-16LE
// in base64.
func encoded(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// sendPowerShell sends an alert through Windows PowerShell, from Windows or
// from WSL. From WSL the variables cross over through WSLENV, which also
// turns the sound file's path into a Windows one.
func sendPowerShell(title, body string, s Settings, wsl bool) error {
	sound := ""
	switch {
	case s.Sound && s.SoundFile != "":
		sound = "file"
	case s.Sound:
		sound = "default"
	}
	popup := "0"
	if s.Popup {
		popup = "1"
	}
	env := append(os.Environ(),
		"KATTASKS_TITLE="+title, "KATTASKS_BODY="+body, "KATTASKS_POPUP="+popup,
		"KATTASKS_SOUND="+sound, "KATTASKS_SOUNDFILE="+s.SoundFile)
	dir := ""
	if wsl {
		share := "KATTASKS_TITLE/u:KATTASKS_BODY/u:KATTASKS_POPUP/u:KATTASKS_SOUND/u:KATTASKS_SOUNDFILE/up"
		if w := os.Getenv("WSLENV"); w != "" {
			share = w + ":" + share
		}
		env = append(env, "WSLENV="+share)
		if _, err := os.Stat("/mnt/c"); err == nil {
			dir = "/mnt/c" // PowerShell warns when started in a WSL folder
		}
	}
	return run("powershell.exe", []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded(psScript)}, env, dir)
}
