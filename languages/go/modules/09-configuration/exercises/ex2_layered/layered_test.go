package layered

import (
	"errors"
	"testing"
	"time"
)

func env(kv map[string]string) Lookup {
	return func(k string) (string, bool) { v, ok := kv[k]; return v, ok }
}

func TestDefaultsWhenNothingIsGiven(t *testing.T) {
	cfg, err := Load(nil, env(nil), nil)
	if err != nil || cfg != Defaults() {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestPrecedence(t *testing.T) {
	file := []byte(`{"addr":":1000","log_level":"debug","timeout":"1s"}`)
	vars := map[string]string{"ADDR": ":2000", "LOG_LEVEL": "warn"}
	args := []string{"-addr", ":3000"}

	tests := []struct {
		name string
		args []string
		env  map[string]string
		file []byte
		want Config
	}{
		{"file only", nil, nil, file, Config{":1000", "debug", time.Second, true}},
		{"env beats file", nil, vars, file, Config{":2000", "warn", time.Second, true}},
		{"flag beats env and file", args, vars, file, Config{":3000", "warn", time.Second, true}},
		{"env without file", nil, vars, nil, Config{":2000", "warn", 5 * time.Second, true}},
		{"flag beats default", []string{"-timeout=250ms"}, nil, nil, Config{"127.0.0.1:8080", "info", 250 * time.Millisecond, true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Load(tc.args, env(tc.env), tc.file)
			if err != nil || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestFlagsLeftOutDoNotResetLowerLayers(t *testing.T) {
	got, err := Load([]string{"-log-level", "error"}, env(map[string]string{"ADDR": ":2000", "TIMEOUT": "9s"}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Addr != ":2000" || got.Timeout != 9*time.Second || got.LogLevel != "error" {
		t.Fatalf("got %+v: unset flags must not override env values", got)
	}
}

func TestFlagEqualToDefaultStillOverrides(t *testing.T) {
	got, err := Load([]string{"-addr", "127.0.0.1:8080"}, env(map[string]string{"ADDR": ":2000"}), nil)
	if err != nil || got.Addr != "127.0.0.1:8080" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestBooleanLayers(t *testing.T) {
	off := []byte(`{"metrics": false}`)
	if got, err := Load(nil, env(nil), off); err != nil || got.Metrics {
		t.Errorf("file false: %+v, %v", got, err)
	}
	if got, err := Load(nil, env(map[string]string{"METRICS": "true"}), off); err != nil || !got.Metrics {
		t.Errorf("env true over file false: %+v, %v", got, err)
	}
	if got, err := Load([]string{"-metrics=false"}, env(map[string]string{"METRICS": "true"}), nil); err != nil || got.Metrics {
		t.Errorf("flag false over env true: %+v, %v", got, err)
	}
	if got, err := Load([]string{"-timeout", "1s"}, env(nil), off); err != nil || got.Metrics {
		t.Errorf("an unrelated flag must not turn metrics back on: %+v, %v", got, err)
	}
}

func TestEmptyEnvironmentValuesAreIgnored(t *testing.T) {
	got, err := Load(nil, env(map[string]string{"ADDR": "", "TIMEOUT": "  ", "METRICS": ""}), nil)
	if err != nil || got != Defaults() {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestErrorsAreErrInvalid(t *testing.T) {
	tests := map[string]struct {
		args []string
		env  map[string]string
		file string
	}{
		"malformed json":      {file: `{"addr":`},
		"unknown file key":    {file: `{"adr":":9"}`},
		"bad file timeout":    {file: `{"timeout":"soon"}`},
		"bad env timeout":     {env: map[string]string{"TIMEOUT": "soon"}},
		"bad env bool":        {env: map[string]string{"METRICS": "maybe"}},
		"unknown flag":        {args: []string{"-verbose"}},
		"bad flag value":      {args: []string{"-timeout", "soon"}},
		"positional argument": {args: []string{"extra"}},
		"bad level":           {env: map[string]string{"LOG_LEVEL": "loud"}},
		"zero timeout":        {env: map[string]string{"TIMEOUT": "0s"}},
		"negative timeout":    {args: []string{"-timeout=-1s"}},
		"addr without port":   {env: map[string]string{"ADDR": "localhost"}},
		"invalid after merge": {file: `{"log_level":"debug"}`, args: []string{"-log-level", "nope"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var file []byte
			if tc.file != "" {
				file = []byte(tc.file)
			}
			got, err := Load(tc.args, env(tc.env), file)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
			if got != (Config{}) {
				t.Errorf("cfg = %+v, want zero value on error", got)
			}
		})
	}
}
