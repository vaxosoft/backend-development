// Package logsetup is exercise 1: build a production-style slog logger from options.
package logsetup

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// ErrBadOption is wrapped by every error New returns for an unusable Options value.
var ErrBadOption = errors.New("bad logging option")

// Options configure New.
type Options struct {
	Format  string // "json" or "text"; empty means "json"
	Level   string // "debug", "info", "warn" or "error" in any letter case; empty means "info"
	Service string // when not empty, added to every record as the attribute "service"
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("%w: level %q", ErrBadOption, s)
}

// New returns a logger writing to w, and the LevelVar that controls it (changing it at runtime must
// change what the logger emits).
//
//   - Unknown formats or levels return an error for which errors.Is(err, ErrBadOption) is true.
//     Only the four level names above are valid (not slog's "INFO+2" forms).
//   - Any attribute whose key is "password" or "token" must be written as "[redacted]", also when it is
//     inside a group (use HandlerOptions.ReplaceAttr).
func New(w io.Writer, o Options) (*slog.Logger, *slog.LevelVar, error) {
	lvl, err := parseLevel(o.Level)
	if err != nil {
		return nil, nil, err
	}
	var level slog.LevelVar
	level.Set(lvl)

	opts := &slog.HandlerOptions{
		Level: &level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == "password" || a.Key == "token" {
				return slog.String(a.Key, "[redacted]")
			}
			return a
		},
	}
	var h slog.Handler
	switch o.Format {
	case "", "json":
		h = slog.NewJSONHandler(w, opts)
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		return nil, nil, fmt.Errorf("%w: format %q", ErrBadOption, o.Format)
	}
	logger := slog.New(h)
	if o.Service != "" {
		logger = logger.With("service", o.Service)
	}
	return logger, &level, nil
}
