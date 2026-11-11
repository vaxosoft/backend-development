package logsetup

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func TestJSONIsTheDefaultAndCarriesService(t *testing.T) {
	var buf bytes.Buffer
	logger, _, err := New(&buf, Options{Service: "tasks"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hello", "id", 7)
	got := lines(t, &buf)
	if len(got) != 1 {
		t.Fatalf("got %d lines: %s", len(got), buf.String())
	}
	m := got[0]
	if m["msg"] != "hello" || m["level"] != "INFO" || m["service"] != "tasks" || m["id"] != float64(7) {
		t.Errorf("record = %v", m)
	}
	if _, ok := m["time"]; !ok {
		t.Error("production logs keep their timestamp")
	}
}

func TestNoServiceAttributeWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	logger, _, _ := New(&buf, Options{})
	logger.Info("x")
	if _, ok := lines(t, &buf)[0]["service"]; ok {
		t.Error("empty Service must not add an attribute")
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger, _, err := New(&buf, Options{Format: "text", Service: "tasks"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Warn("careful")
	out := buf.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "service=tasks") || strings.HasPrefix(out, "{") {
		t.Errorf("output = %q", out)
	}
}

func TestLevelsFilterAndCanChangeAtRuntime(t *testing.T) {
	var buf bytes.Buffer
	logger, lv, err := New(&buf, Options{Level: "WARN"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hidden")
	logger.Warn("shown")
	if got := lines(t, &buf); len(got) != 1 || got[0]["msg"] != "shown" {
		t.Fatalf("lines = %v", got)
	}

	buf.Reset()
	lv.Set(slog.LevelDebug)
	logger.Debug("now visible")
	if got := lines(t, &buf); len(got) != 1 || got[0]["level"] != "DEBUG" {
		t.Fatalf("after Set(Debug): %v", got)
	}
}

func TestEveryNamedLevelIsAccepted(t *testing.T) {
	for _, l := range []string{"", "debug", "info", "warn", "error", "Debug", "ERROR"} {
		if _, _, err := New(&bytes.Buffer{}, Options{Level: l}); err != nil {
			t.Errorf("level %q: %v", l, err)
		}
	}
}

func TestBadOptions(t *testing.T) {
	for _, o := range []Options{{Format: "xml"}, {Format: "JSON "}, {Level: "loud"}, {Level: "INFO+2"}, {Level: "warning"}} {
		logger, lv, err := New(&bytes.Buffer{}, o)
		if !errors.Is(err, ErrBadOption) {
			t.Errorf("%+v: err = %v, want ErrBadOption", o, err)
		}
		if logger != nil || lv != nil {
			t.Errorf("%+v: want nil results on error", o)
		}
	}
}

func TestSensitiveKeysAreRedacted(t *testing.T) {
	var buf bytes.Buffer
	logger, _, _ := New(&buf, Options{})
	logger.Info("login", "user", "ann", "password", "hunter2", slog.Group("req", slog.String("token", "abc123"), slog.String("path", "/x")))
	logger.With("token", "xyz789").Info("again")

	out := buf.String()
	for _, secret := range []string{"hunter2", "abc123", "xyz789"} {
		if strings.Contains(out, secret) {
			t.Errorf("output leaks %q: %s", secret, out)
		}
	}
	first := lines(t, &buf)[0]
	req := first["req"].(map[string]any)
	if first["password"] != "[redacted]" || req["token"] != "[redacted]" || req["path"] != "/x" || first["user"] != "ann" {
		t.Errorf("record = %v", first)
	}
}
