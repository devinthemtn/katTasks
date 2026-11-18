package main

import (
	"runtime/debug"
	"strings"
)

// version is set by release builds: -ldflags "-X main.version=0.3.0".
var version = ""

// Version is the katTasks release: the one stamped in by a release build,
// else the module version when installed with go install …@vX.Y.Z, else
// "dev" for a local build.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return "dev"
}
