package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

const token = "hunter2-hunter2-hunter2"

func valid() Settings {
	return Settings{
		Addr:          "127.0.0.1:8080",
		Env:           "dev",
		APIToken:      Secret(token),
		UpstreamURL:   "https://api.example.com/v1",
		Workers:       4,
		ShutdownGrace: 10 * time.Second,
	}
}

func TestValidSettings(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	s := valid()
	s.UpstreamURL = "" // optional
	if err := s.Validate(); err != nil {
		t.Fatalf("empty upstream must be valid: %v", err)
	}
}

func TestValidateReportsEveryProblemInOrder(t *testing.T) {
	s := Settings{Addr: "localhost", Env: "qa", APIToken: "short", UpstreamURL: "ftp://x", Workers: 0, ShutdownGrace: 0}
	err := s.Validate()
	if err == nil {
		t.Fatal("want an error")
	}
	var fields []string
	for _, p := range Problems(err) {
		fields = append(fields, p.Field)
		if strings.Contains(p.Problem, "short") {
			t.Errorf("problem for %s leaks the value: %q", p.Field, p.Problem)
		}
	}
	want := "addr env api_token upstream_url workers shutdown_grace"
	if got := strings.Join(fields, " "); got != want {
		t.Fatalf("fields = %q, want %q", got, want)
	}

	var fe FieldError
	if !errors.As(err, &fe) || fe.Field != "addr" {
		t.Errorf("errors.As should find the first FieldError, got %+v", fe)
	}
}

func TestValidateBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Settings)
		field  string // "" = valid
	}{
		{"port 0", func(s *Settings) { s.Addr = ":0" }, "addr"},
		{"port too high", func(s *Settings) { s.Addr = ":65536" }, "addr"},
		{"port max", func(s *Settings) { s.Addr = ":65535" }, ""},
		{"token 15", func(s *Settings) { s.APIToken = Secret(strings.Repeat("x", 15)) }, "api_token"},
		{"token 16", func(s *Settings) { s.APIToken = Secret(strings.Repeat("x", 16)) }, ""},
		{"workers 64", func(s *Settings) { s.Workers = 64 }, ""},
		{"workers 65", func(s *Settings) { s.Workers = 65 }, "workers"},
		{"grace 5m", func(s *Settings) { s.ShutdownGrace = 5 * time.Minute }, ""},
		{"grace over 5m", func(s *Settings) { s.ShutdownGrace = 5*time.Minute + 1 }, "shutdown_grace"},
		{"relative upstream", func(s *Settings) { s.UpstreamURL = "/v1" }, "upstream_url"},
		{"upstream without host", func(s *Settings) { s.UpstreamURL = "http://" }, "upstream_url"},
		{"prod env", func(s *Settings) { s.Env = "prod" }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.mutate(&s)
			ps := Problems(s.Validate())
			switch {
			case tc.field == "" && len(ps) != 0:
				t.Fatalf("unexpected problems: %v", ps)
			case tc.field != "" && (len(ps) != 1 || ps[0].Field != tc.field):
				t.Fatalf("problems = %v, want exactly one for %s", ps, tc.field)
			}
		})
	}
}

func TestProblemsLooksThroughWrappingAndNestedJoins(t *testing.T) {
	a, b, c := FieldError{"a", "x"}, FieldError{"b", "y"}, FieldError{"c", "z"}
	err := fmt.Errorf("loading config: %w", errors.Join(a, errors.Join(errors.New("plain"), b), fmt.Errorf("ctx: %w", c)))
	got := Problems(err)
	if len(got) != 3 || got[0] != a || got[1] != b || got[2] != c {
		t.Fatalf("Problems = %v", got)
	}
	if Problems(nil) != nil || Problems(errors.New("plain")) != nil {
		t.Error("no FieldError inside: want nil")
	}
}

func TestSecretIsRedactedEverywhere(t *testing.T) {
	s := valid()
	var jsonLog, textLog bytes.Buffer
	jl := slog.New(slog.NewJSONHandler(&jsonLog, nil))
	tl := slog.New(slog.NewTextHandler(&textLog, nil))
	jl.Info("direct", "token", s.APIToken)
	jl.Info("struct", "settings", s)
	tl.Info("direct", "token", s.APIToken)
	tl.Info("struct", "settings", s)

	j, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"%v":        fmt.Sprintf("%v", s.APIToken),
		"%s":        fmt.Sprintf("%s", s.APIToken),
		"%q":        fmt.Sprintf("%q", s.APIToken),
		"%#v":       fmt.Sprintf("%#v", s.APIToken),
		"%+v":       fmt.Sprintf("%+v", s),
		"%#v of s":  fmt.Sprintf("%#v", s),
		"json":      string(j),
		"slog json": jsonLog.String(),
		"slog text": textLog.String(),
	}
	for name, out := range outputs {
		if strings.Contains(out, token) {
			t.Errorf("%s leaks the secret: %s", name, out)
		}
		if !strings.Contains(out, "[redacted]") {
			t.Errorf("%s should show [redacted]: %s", name, out)
		}
	}
	if s.APIToken.Reveal() != token {
		t.Error("Reveal must return the real value")
	}
}

func TestEmptySecretStaysEmpty(t *testing.T) {
	var s Secret
	j, _ := json.Marshal(s)
	if s.String() != "" || string(j) != `""` || fmt.Sprintf("%v", s) != "" {
		t.Errorf("empty secret: String=%q json=%s", s.String(), j)
	}
}
