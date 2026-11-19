package alert

func send(title, body string, s Settings) error { return sendPowerShell(title, body, s, false) }
