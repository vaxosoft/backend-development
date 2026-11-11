// Package buildinfo is exercise 1: turn link-time flags and debug.BuildInfo into a version report.
package buildinfo

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"
)

// Info describes the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	Modified  bool   `json:"modified"`
}

// FromBuildInfo combines the -ldflags version with what the toolchain recorded.
func FromBuildInfo(ldflagsVersion string, bi *debug.BuildInfo) Info {
	info := Info{Version: ldflagsVersion}
	if bi != nil {
		info.GoVersion = bi.GoVersion
		if info.Version == "" || info.Version == "dev" {
			if v := bi.Main.Version; v != "" && v != "(devel)" {
				info.Version = v
			}
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
				if len(info.Commit) > 7 {
					info.Commit = info.Commit[:7]
				}
			case "vcs.time":
				info.BuildTime = s.Value
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	return info
}

// String renders "1.2.3 (commit abc1234-dirty, built 2030-01-01T00:00:00Z, go1.27.2)".
func (i Info) String() string {
	var parts []string
	if i.Commit != "" {
		c := "commit " + i.Commit
		if i.Modified {
			c += "-dirty"
		}
		parts = append(parts, c)
	}
	if i.BuildTime != "" {
		parts = append(parts, "built "+i.BuildTime)
	}
	if i.GoVersion != "" {
		parts = append(parts, i.GoVersion)
	}
	if len(parts) == 0 {
		return i.Version
	}
	return i.Version + " (" + strings.Join(parts, ", ") + ")"
}

// Handler serves info as JSON on GET requests.
func Handler(i Info) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(i)
	})
}
