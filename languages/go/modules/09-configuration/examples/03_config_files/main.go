// Example 03: JSON and YAML config files, decoded strictly so typos are caught.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Addr     string   `json:"addr" yaml:"addr"`
	LogLevel string   `json:"log_level" yaml:"log_level"`
	Timeout  Duration `json:"timeout" yaml:"timeout"`
	Tags     []string `json:"tags" yaml:"tags"`
}

// Duration reads "1500ms" / "5s" from both JSON and YAML (time.Duration alone would expect nanoseconds).
type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	*d = Duration(v)
	return err
}

func defaults() Config {
	return Config{Addr: "127.0.0.1:8080", LogLevel: "info", Timeout: Duration(5 * time.Second)}
}

func parseJSON(data []byte) (Config, error) {
	cfg := defaults() // absent keys keep their defaults
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return cfg, dec.Decode(&cfg)
}

func parseYAML(data []byte) (Config, error) {
	cfg := defaults()
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return cfg, dec.Decode(&cfg)
}

func show(label string, c Config, err error) {
	if err != nil {
		fmt.Printf("%-14s error: %v\n", label, err)
		return
	}
	fmt.Printf("%-14s addr=%s level=%s timeout=%s tags=%v\n", label, c.Addr, c.LogLevel, time.Duration(c.Timeout), c.Tags)
}

func main() {
	jsonDoc := `{"addr": ":9000", "timeout": "2s", "tags": ["a", "b"]}`
	yamlDoc := strings.TrimSpace(`
addr: ":9100"
log_level: debug
timeout: 750ms
tags: [docs]
`)

	c, err := parseJSON([]byte(jsonDoc))
	show("json", c, err)
	c, err = parseYAML([]byte(yamlDoc))
	show("yaml", c, err)

	// A typo must fail loudly instead of silently using the default.
	c, err = parseJSON([]byte(`{"adr": ":9000"}`))
	show("json typo", c, err)
	c, err = parseYAML([]byte("adr: \":9000\"\n"))
	show("yaml typo", c, err)

	// Without strictness the typo is silently ignored.
	loose := defaults()
	err = json.Unmarshal([]byte(`{"adr": ":9000"}`), &loose)
	fmt.Printf("%-14s addr=%s err=%v (typo ignored!)\n", "json loose", loose.Addr, err)

	c, err = parseJSON([]byte(`{"timeout": "soon"}`))
	show("bad duration", c, err)
}
