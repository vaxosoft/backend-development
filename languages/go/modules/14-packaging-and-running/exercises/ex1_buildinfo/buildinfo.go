// Package buildinfo is exercise 1: turn link-time flags and debug.BuildInfo into a version report.
//
// Run the tests: go test ./modules/14-packaging-and-running/exercises/ex1_buildinfo/
package buildinfo

import (
	"net/http"
	"runtime/debug"
)

// Info describes the running binary. The JSON names are the wire format of the version endpoint.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	Modified  bool   `json:"modified"`
}

// FromBuildInfo combines the -ldflags version with what the toolchain recorded (bi may be nil).
//
//   - Version: ldflagsVersion unless it is "" or "dev"; then bi.Main.Version unless that is ""
//     or "(devel)"; otherwise "dev".
//   - Commit: setting vcs.revision shortened to its first 7 characters.
//   - BuildTime: setting vcs.time. Modified: setting vcs.modified == "true".
//   - GoVersion: bi.GoVersion.
func FromBuildInfo(ldflagsVersion string, bi *debug.BuildInfo) Info {
	return Info{} // TODO
}

// String renders "1.2.3 (commit abc1234-dirty, built 2030-01-01T00:00:00Z, go1.27.2)".
// Missing parts are left out, and with no parts at all only the version is returned.
// "-dirty" is appended to the commit when Modified is set.
func (i Info) String() string {
	return "" // TODO
}

// Handler serves i as a JSON object on GET with Content-Type application/json.
// Any other method gets 405 and an Allow: GET header.
func Handler(i Info) http.Handler {
	return http.NotFoundHandler() // TODO
}
