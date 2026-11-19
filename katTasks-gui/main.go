// Command katTasks-gui is the desktop app for katTasks. It runs the same
// task list as the terminal app (the ui package's model, driven without a
// terminal) and draws it in a window, so keys, chords and settings are the
// same in both.
package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/devinthemtn/katTasks/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

// version is set by release builds: -ldflags "-X main.version=0.3.0".
var version = ""

func appVersion() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "dev"
}

func main() {
	cfgPath := flag.String("config", config.DefaultPath(), "settings file")
	todoFile := flag.String("file", "", "todo.txt to open (overrides files.todo)")
	showVersion := flag.Bool("version", false, "print the version, then exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("katTasks-gui", appVersion())
		return
	}

	quietStartup()

	var app *application.App
	svc := NewTasksService(*cfgPath, *todoFile, appVersion(), func() *application.App { return app })

	app = application.New(application.Options{
		Name:        "katTasks-gui",
		Description: "Keyboard-driven todo.txt task manager",
		Icon:        icon,
		Services:    []application.Service{application.NewService(svc)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Linux:       application.LinuxOptions{ProgramName: "katTasks-gui"},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "katTasks",
		Width:            1100,
		Height:           720,
		MinWidth:         640,
		MinHeight:        400,
		BackgroundColour: application.NewRGB(24, 24, 31),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
