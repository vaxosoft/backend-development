package starter

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalidConfig wraps every configuration problem LoadConfig reports.
var ErrInvalidConfig = errors.New("invalid configuration")

var (
	logLevels    = []string{"debug", "info", "warn", "error"}
	environments = []string{"development", "test", "production"}
)

// Config is the service configuration, read from the environment.
type Config struct {
	Port     int    // PORT, 1-65535
	LogLevel string // LOG_LEVEL: debug|info|warn|error
	Env      string // ENV: development|test|production
}

// DefaultConfig returns the values used when no variable is set.
func DefaultConfig() Config {
	return Config{Port: 8080, LogLevel: "info", Env: "development"}
}

// SlogLevel converts LogLevel to a slog.Level.
func (c Config) SlogLevel() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return slog.LevelInfo
	}
	return level
}

// LoadConfig builds a Config from getenv (os.Getenv in production). Unset or
// empty variables keep their defaults. All problems are reported together.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := DefaultConfig()
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("%w: "+format, append([]any{ErrInvalidConfig}, args...)...))
	}

	if v := getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			bad("PORT %q must be an integer between 1 and 65535", v)
		} else {
			cfg.Port = port
		}
	}
	if v := strings.ToLower(getenv("LOG_LEVEL")); v != "" {
		if slices.Contains(logLevels, v) {
			cfg.LogLevel = v
		} else {
			bad("LOG_LEVEL %q must be one of %s", v, strings.Join(logLevels, ", "))
		}
	}
	if v := strings.ToLower(getenv("ENV")); v != "" {
		if slices.Contains(environments, v) {
			cfg.Env = v
		} else {
			bad("ENV %q must be one of %s", v, strings.Join(environments, ", "))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
