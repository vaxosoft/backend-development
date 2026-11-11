package starter

import (
	"errors"
	"log/slog"
)

// ErrInvalidConfig wraps every configuration problem LoadConfig reports.
var ErrInvalidConfig = errors.New("invalid configuration")

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

// LoadConfig builds a Config from getenv (os.Getenv in production).
//
// TODO: start from DefaultConfig and override from PORT, LOG_LEVEL and ENV.
// Unset or empty variables keep their defaults. Reject PORT outside 1-65535,
// a LOG_LEVEL other than debug|info|warn|error (case-insensitive) and an ENV
// other than development|test|production. Report every problem together,
// each wrapping ErrInvalidConfig and naming the variable.
func LoadConfig(_ func(string) string) (Config, error) {
	return DefaultConfig(), nil
}
