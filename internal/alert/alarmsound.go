package alert

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"

	"github.com/devinthemtn/katTasks/internal/config"
)

// The built-in alarm is a classic alarm clock: four quick beeps and a
// pause, about a second, made here rather than shipped as a file. It's
// written as a .wav because every OS's player reads that.
const (
	sampleRate = 22050
	beepHz     = 1000.0
	beepLen    = 0.075 // seconds
	beepGap    = 0.055
	beepsPer   = 4
	pause      = 0.45
)

// alarmWAV is one beep-beep-beep-beep cycle.
func alarmWAV() []byte {
	var samples []int16
	add := func(secs float64, tone bool) {
		n := int(secs * float64(sampleRate))
		fade := sampleRate / 250 // 4ms, so the edges don't click
		for i := 0; i < n; i++ {
			v := 0.0
			if tone {
				ph := 2 * math.Pi * beepHz * float64(i) / sampleRate
				// A soft square wave: the fundamental plus odd harmonics,
				// brighter and more piercing than a sine.
				v = math.Sin(ph) + math.Sin(3*ph)/3 + math.Sin(5*ph)/5
				env := 1.0
				if i < fade {
					env = float64(i) / float64(fade)
				} else if n-i < fade {
					env = float64(n-i) / float64(fade)
				}
				v *= 0.45 * env
			}
			samples = append(samples, int16(v*math.MaxInt16))
		}
	}
	for i := 0; i < beepsPer; i++ {
		add(beepLen, true)
		add(beepGap, false)
	}
	add(pause, false)

	var b bytes.Buffer
	data := len(samples) * 2
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	w(uint32(36 + data))
	b.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1)) // PCM
	w(uint16(1)) // mono
	w(uint32(sampleRate))
	w(uint32(sampleRate * 2))
	w(uint16(2))
	w(uint16(16))
	b.WriteString("data")
	w(uint32(data))
	w(samples)
	return b.Bytes()
}

// AlarmSoundFile returns the built-in alarm as a file in the user cache
// folder, writing it if it isn't there or has changed.
func AlarmSoundFile() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	p := filepath.Join(base, config.AppName, "alarm.wav")
	want := alarmWAV()
	if have, err := os.ReadFile(p); err == nil && bytes.Equal(have, want) {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	return p, os.WriteFile(p, want, 0o644)
}
