// Package logsetup is exercise 1: build a production-style slog logger from options.
package logsetup

import (
	"errors"
	"io"
	"log/slog"
)

// ErrBadOption is wrapped by every error New returns for an unusable Options value.
var ErrBadOption = errors.New("bad logging option")

// Options configure New.
type Options struct {
	Format  string // "json" or "text"; empty means "json"
	Level   string // "debug", "info", "warn" or "error" in any letter case; empty means "info"
	Service string // when not empty, added to every record as the attribute "service"
}

// New returns a logger writing to w, and the LevelVar that controls it (changing it at runtime must
// change what the logger emits).
//
//   - Unknown formats or levels return an error for which errors.Is(err, ErrBadOption) is true.
//     Only the four level names above are valid (not slog's "INFO+2" forms).
//   - Any attribute whose key is "password" or "token" must be written as "[redacted]", also when it is
//     inside a group (use HandlerOptions.ReplaceAttr).
func New(w io.Writer, o Options) (*slog.Logger, *slog.LevelVar, error) {
	panic("TODO")
}
