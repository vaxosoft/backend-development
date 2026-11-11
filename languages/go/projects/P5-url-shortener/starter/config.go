package shortener

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// ConfigFromEnv reads JWT_SECRET (required, at least 16 bytes), DB_PATH,
// RATE_BURST and RATE_REFILL (tokens per second). Logs go to stdout.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	secret := getenv("JWT_SECRET")
	if len(secret) < 16 {
		return Config{}, errors.New("JWT_SECRET is required and must be at least 16 bytes")
	}
	cfg := DefaultConfig([]byte(secret))
	cfg.LogWriter = os.Stdout
	if v := getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := getenv("RATE_BURST"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("RATE_BURST must be a positive integer, got %q", v)
		}
		cfg.RateBurst = n
	}
	if v := getenv("RATE_REFILL"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 {
			return Config{}, fmt.Errorf("RATE_REFILL must be a positive number, got %q", v)
		}
		cfg.RateRefillPerSecond = f
	}
	return cfg, nil
}
