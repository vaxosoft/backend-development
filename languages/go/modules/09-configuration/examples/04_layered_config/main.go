// Example 04: layered configuration: defaults < file < environment < flags, with the source of every value.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Secret string

func (Secret) String() string { return "[redacted]" }

type Config struct {
	Addr           string
	LogLevel       string
	RequestTimeout time.Duration
	APIToken       Secret
}

// Sources records which layer supplied each field.
type Sources map[string]string

type fileConfig struct {
	Addr           *string `yaml:"addr"`
	LogLevel       *string `yaml:"log_level"`
	RequestTimeout *string `yaml:"request_timeout"`
}

type Lookup func(string) (string, bool)

func defaults() (Config, Sources) {
	return Config{Addr: "127.0.0.1:8080", LogLevel: "info", RequestTimeout: 5 * time.Second},
		Sources{"Addr": "default", "LogLevel": "default", "RequestTimeout": "default", "APIToken": "default"}
}

// Load applies the layers in order of increasing priority. The file is optional.
func Load(args []string, lookup Lookup, readFile func(string) ([]byte, error)) (Config, Sources, error) {
	cfg, src := defaults()

	// Flags are parsed first only to learn the file name; they are applied last.
	fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("config", "", "config file")
	addr := fs.String("addr", "", "listen address")
	level := fs.String("log-level", "", "log level")
	timeout := fs.Duration("request-timeout", 0, "request timeout")
	if err := fs.Parse(args); err != nil {
		return Config{}, nil, err
	}

	var errs []error
	if *file != "" {
		if err := applyFile(&cfg, src, *file, readFile); err != nil {
			errs = append(errs, err)
		}
	}
	if err := applyEnv(&cfg, src, lookup); err != nil {
		errs = append(errs, err)
	}
	// Visit only sees flags that were actually passed, so an unset flag never overrides the env.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr, src["Addr"] = *addr, "flag -addr"
		case "log-level":
			cfg.LogLevel, src["LogLevel"] = *level, "flag -log-level"
		case "request-timeout":
			cfg.RequestTimeout, src["RequestTimeout"] = *timeout, "flag -request-timeout"
		}
	})

	if err := cfg.Validate(); err != nil {
		errs = append(errs, err)
	}
	return cfg, src, errors.Join(errs...)
}

func applyFile(cfg *Config, src Sources, path string, readFile func(string) ([]byte, error)) error {
	data, err := readFile(path)
	if err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}
	var fc fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil {
		return fmt.Errorf("config file %s: %w", path, err)
	}
	origin := "file " + path
	if fc.Addr != nil {
		cfg.Addr, src["Addr"] = *fc.Addr, origin
	}
	if fc.LogLevel != nil {
		cfg.LogLevel, src["LogLevel"] = *fc.LogLevel, origin
	}
	if fc.RequestTimeout != nil {
		d, err := time.ParseDuration(*fc.RequestTimeout)
		if err != nil {
			return fmt.Errorf("config file %s: request_timeout: %w", path, err)
		}
		cfg.RequestTimeout, src["RequestTimeout"] = d, origin
	}
	return nil
}

func applyEnv(cfg *Config, src Sources, lookup Lookup) error {
	if v, ok := lookup("ADDR"); ok && v != "" {
		cfg.Addr, src["Addr"] = v, "env ADDR"
	}
	if v, ok := lookup("LOG_LEVEL"); ok && v != "" {
		cfg.LogLevel, src["LogLevel"] = v, "env LOG_LEVEL"
	}
	if v, ok := lookup("API_TOKEN"); ok && v != "" {
		cfg.APIToken, src["APIToken"] = Secret(v), "env API_TOKEN"
	}
	if v, ok := lookup("REQUEST_TIMEOUT"); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("REQUEST_TIMEOUT: %w", err)
		}
		cfg.RequestTimeout, src["RequestTimeout"] = d, "env REQUEST_TIMEOUT"
	}
	return nil
}

// Validate reports every problem at once, not just the first.
func (c Config) Validate() error {
	var errs []error
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("log level %q: want debug|info|warn|error", c.LogLevel))
	}
	if c.RequestTimeout <= 0 || c.RequestTimeout > time.Minute {
		errs = append(errs, fmt.Errorf("request timeout %s: want 1ns..1m", c.RequestTimeout))
	}
	if _, port, err := net.SplitHostPort(c.Addr); err != nil {
		errs = append(errs, fmt.Errorf("addr %q: want host:port", c.Addr))
	} else if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		errs = append(errs, fmt.Errorf("addr %q: port out of range", c.Addr))
	}
	return errors.Join(errs...)
}

func report(title string, args []string, env map[string]string, files map[string]string) {
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	read := func(p string) ([]byte, error) {
		if s, ok := files[p]; ok {
			return []byte(s), nil
		}
		return nil, os.ErrNotExist
	}
	cfg, src, err := Load(args, lookup, read)
	fmt.Println("==", title)
	if err != nil {
		fmt.Println("invalid configuration:")
		fmt.Println(err)
		return
	}
	fmt.Printf("addr            = %-16v (%s)\n", cfg.Addr, src["Addr"])
	fmt.Printf("log level       = %-16v (%s)\n", cfg.LogLevel, src["LogLevel"])
	fmt.Printf("request timeout = %-16v (%s)\n", cfg.RequestTimeout, src["RequestTimeout"])
	fmt.Printf("api token       = %-16v (%s)\n", cfg.APIToken, src["APIToken"])
}

func main() {
	const file = "addr: \":9000\"\nlog_level: debug\nrequest_timeout: 2s\n"
	files := map[string]string{"app.yaml": file}

	report("defaults only", nil, nil, nil)
	report("file", []string{"-config", "app.yaml"}, nil, files)
	report("file + env", []string{"-config", "app.yaml"},
		map[string]string{"LOG_LEVEL": "warn", "API_TOKEN": "s3cret"}, files)
	report("file + env + flag", []string{"-config", "app.yaml", "-addr", "127.0.0.1:7000"},
		map[string]string{"ADDR": ":8000", "LOG_LEVEL": "warn"}, files)
	report("invalid", []string{"-addr", "nowhere", "-request-timeout", "2h"},
		map[string]string{"LOG_LEVEL": "loud"}, nil)
	report("missing file", []string{"-config", "nope.yaml"}, nil, files)
}
