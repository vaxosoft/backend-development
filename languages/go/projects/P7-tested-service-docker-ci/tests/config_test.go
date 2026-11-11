package tests

import (
	"errors"
	"strings"
	"testing"

	"webdev/projects/P7-tested-service-docker-ci/starter"
)

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestLoadConfigUsesDefaultsWhenNothingIsSet(t *testing.T) {
	cfg, err := starter.LoadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 || cfg.LogLevel != "info" || cfg.Env != "development" {
		t.Errorf("cfg = %+v, want port 8080, log level info, env development", cfg)
	}
}

func TestLoadConfigReadsTheEnvironment(t *testing.T) {
	cfg, err := starter.LoadConfig(env(map[string]string{"PORT": "9090", "LOG_LEVEL": "debug", "ENV": "production"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9090 || cfg.LogLevel != "debug" || cfg.Env != "production" {
		t.Errorf("cfg = %+v, want port 9090, log level debug, env production", cfg)
	}
}

func TestLoadConfigIgnoresCaseOfLogLevelAndEnv(t *testing.T) {
	cfg, err := starter.LoadConfig(env(map[string]string{"LOG_LEVEL": "WARN", "ENV": "Test"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "warn" || cfg.Env != "test" {
		t.Errorf("cfg = %+v, want normalised lower-case values", cfg)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		vars map[string]string
		want string // variable the message must name
	}{
		{"port not a number", map[string]string{"PORT": "http"}, "PORT"},
		{"port zero", map[string]string{"PORT": "0"}, "PORT"},
		{"port too large", map[string]string{"PORT": "70000"}, "PORT"},
		{"unknown log level", map[string]string{"LOG_LEVEL": "verbose"}, "LOG_LEVEL"},
		{"unknown environment", map[string]string{"ENV": "prod"}, "ENV"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := starter.LoadConfig(env(tc.vars))
			if !errors.Is(err, starter.ErrInvalidConfig) {
				t.Fatalf("err = %v, want it to wrap ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to name %s", err, tc.want)
			}
		})
	}
}

func TestLoadConfigReportsEveryProblemAtOnce(t *testing.T) {
	_, err := starter.LoadConfig(env(map[string]string{"PORT": "-1", "LOG_LEVEL": "loud", "ENV": "mars"}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, name := range []string{"PORT", "LOG_LEVEL", "ENV"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("err = %q, want it to mention %s", err, name)
		}
	}
}
