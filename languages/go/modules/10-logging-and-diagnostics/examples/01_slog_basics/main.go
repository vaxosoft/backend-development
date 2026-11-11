// Example 01: log/slog basics: handlers, levels, key-value attributes, With, groups, dynamic level.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"
)

// dropTime removes the timestamp so the output is identical on every run (do not do this in production).
func dropTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return a
}

type user struct {
	ID       int
	Email    string
	Password string
}

// LogValue controls how the type appears in logs, so the password can never be printed by accident.
func (u user) LogValue() slog.Value {
	return slog.GroupValue(slog.Int("id", u.ID), slog.String("email", u.Email))
}

func main() {
	opts := &slog.HandlerOptions{ReplaceAttr: dropTime}

	text := slog.New(slog.NewTextHandler(os.Stdout, opts))
	text.Info("task created", "id", 7, "title", "Write docs")
	text.Debug("not shown: default level is info")
	text.Warn("slow query", "duration", 1500*time.Millisecond)
	text.Error("save failed", "err", errors.New("disk full"))

	// The JSON handler emits one machine-readable object per line.
	js := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	js.Info("task created", "id", 7, "tags", []string{"docs", "p1"})

	// With attaches attributes to every later line; WithGroup nests them.
	reqLog := js.With("request_id", "r-42").WithGroup("http")
	reqLog.Info("handled", "method", "GET", "status", 200)

	// Typed attributes cannot suffer from an odd number of key/value arguments (go vet checks that too).
	js.LogAttrs(context.Background(), slog.LevelInfo, "typed", slog.Int("n", 3), slog.Group("db", slog.String("name", "tasks"), slog.Int("rows", 2)))

	// A LogValuer type decides what is safe to print.
	js.Info("login", "user", user{ID: 1, Email: "a@example.com", Password: "hunter2"})

	// The level can change while the program runs (for example from an admin endpoint).
	var level slog.LevelVar
	dyn := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level, ReplaceAttr: dropTime}))
	dyn.Debug("hidden")
	level.Set(slog.LevelDebug)
	dyn.Debug("visible now")
}
