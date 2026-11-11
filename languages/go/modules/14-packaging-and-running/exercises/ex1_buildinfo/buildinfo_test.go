package buildinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"testing"
)

func settings(kv ...string) []debug.BuildSetting {
	var out []debug.BuildSetting
	for i := 0; i < len(kv); i += 2 {
		out = append(out, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
	}
	return out
}

func TestFromBuildInfo(t *testing.T) {
	full := &debug.BuildInfo{
		GoVersion: "go1.27.2",
		Main:      debug.Module{Version: "v0.4.0"},
		Settings: settings(
			"vcs.revision", "0123456789abcdef", "vcs.time", "2030-01-01T00:00:00Z", "vcs.modified", "true", "CGO_ENABLED", "0"),
	}
	tests := []struct {
		name    string
		version string
		bi      *debug.BuildInfo
		want    Info
	}{
		{"nil build info", "", nil, Info{Version: "dev"}},
		{"ldflags version wins", "1.2.3", full, Info{"1.2.3", "0123456", "2030-01-01T00:00:00Z", "go1.27.2", true}},
		{"dev falls back to module version", "dev", full, Info{"v0.4.0", "0123456", "2030-01-01T00:00:00Z", "go1.27.2", true}},
		{"empty falls back to module version", "", full, Info{"v0.4.0", "0123456", "2030-01-01T00:00:00Z", "go1.27.2", true}},
		{"(devel) is not a version", "dev", &debug.BuildInfo{GoVersion: "go1.27.2", Main: debug.Module{Version: "(devel)"}}, Info{Version: "dev", GoVersion: "go1.27.2"}},
		{"short revision kept", "1.0.0", &debug.BuildInfo{Settings: settings("vcs.revision", "abc", "vcs.modified", "false")}, Info{Version: "1.0.0", Commit: "abc"}},
		{"nil with ldflags", "2.0.0", nil, Info{Version: "2.0.0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromBuildInfo(tc.version, tc.bi); got != tc.want {
				t.Fatalf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		info Info
		want string
	}{
		{Info{Version: "dev"}, "dev"},
		{Info{"1.2.3", "abc1234", "2030-01-01T00:00:00Z", "go1.27.2", false}, "1.2.3 (commit abc1234, built 2030-01-01T00:00:00Z, go1.27.2)"},
		{Info{"1.2.3", "abc1234", "", "go1.27.2", true}, "1.2.3 (commit abc1234-dirty, go1.27.2)"},
		{Info{Version: "1.2.3", GoVersion: "go1.27.2"}, "1.2.3 (go1.27.2)"},
		{Info{Version: "1.2.3", Modified: true}, "1.2.3"},
	}
	for _, tc := range tests {
		if got := tc.info.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

func TestHandler(t *testing.T) {
	want := Info{"1.2.3", "abc1234", "2030-01-01T00:00:00Z", "go1.27.2", true}
	rec := httptest.NewRecorder()
	Handler(want).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if got["version"] != "1.2.3" || got["commit"] != "abc1234" || got["build_time"] != "2030-01-01T00:00:00Z" ||
		got["go_version"] != "go1.27.2" || got["modified"] != true {
		t.Fatalf("body = %v", got)
	}
}

func TestHandlerRejectsOtherMethods(t *testing.T) {
	for _, m := range []string{http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		Handler(Info{Version: "x"}).ServeHTTP(rec, httptest.NewRequest(m, "/version", nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodGet {
			t.Errorf("%s: status %d, Allow %q", m, rec.Code, rec.Header().Get("Allow"))
		}
	}
}
