package aggregator

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ConfigFromEnv reads UPSTREAM_URL (required), MAX_CONCURRENCY, MAX_RETRIES,
// CALL_TIMEOUT, OVERALL_TIMEOUT and BACKOFF_BASE (Go durations such as 500ms).
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	base := getenv("UPSTREAM_URL")
	if base == "" {
		return Config{}, errors.New("UPSTREAM_URL is required")
	}
	cfg := DefaultConfig(base)

	if v := getenv("MAX_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("MAX_CONCURRENCY must be a positive integer, got %q", v)
		}
		cfg.MaxConcurrency = n
	}
	if v := getenv("MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("MAX_RETRIES must be zero or more, got %q", v)
		}
		cfg.MaxRetries = n
	}
	for name, dst := range map[string]*time.Duration{
		"CALL_TIMEOUT":    &cfg.CallTimeout,
		"OVERALL_TIMEOUT": &cfg.OverallTimeout,
		"BACKOFF_BASE":    &cfg.BackoffBase,
	} {
		if v := getenv(name); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive duration, got %q", name, v)
			}
			*dst = d
		}
	}
	return cfg, nil
}
