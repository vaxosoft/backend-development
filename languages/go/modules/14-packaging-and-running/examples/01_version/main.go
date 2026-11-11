// Stamping a binary with its version: -ldflags -X for release builds, debug.ReadBuildInfo for everything else.
package main

import (
	"fmt"
	"runtime/debug"
)

// Overwritten at link time: go build -ldflags "-X main.version=1.2.3 -X main.commit=abc123".
// -X only works on package-level string variables that are not initialised by a function call.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Printf("version=%s commit=%s date=%s\n", version, commit, date)

	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("no build info (binary built without module support)")
		return
	}
	fmt.Println("main module:", info.Main.Path)
	fmt.Println("package:", info.Path)

	// go build stamps these from the checkout (not go run, not outside a repository).
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if rev, ok := settings["vcs.revision"]; ok {
		fmt.Println("vcs revision:", rev, "modified:", settings["vcs.modified"])
	} else {
		fmt.Println("vcs revision: not stamped")
	}
}
