// Example 01: environment variables: Getenv vs LookupEnv, typed parsing, injectable lookup.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Lookup has the signature of os.LookupEnv, so tests can pass a map-backed fake.
type Lookup func(key string) (string, bool)

func intVar(lookup Lookup, key string, def int) (int, error) {
	raw, ok := lookup(key)
	if !ok || raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not an integer", key, raw)
	}
	return n, nil
}

func durationVar(lookup Lookup, key string, def time.Duration) (time.Duration, error) {
	raw, ok := lookup(key)
	if !ok || raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a duration (try 500ms, 5s, 2m)", key, raw)
	}
	return d, nil
}

func main() {
	os.Setenv("DEMO_EMPTY", "")
	os.Setenv("DEMO_PORT", "9090")
	os.Setenv("DEMO_TIMEOUT", "1500ms")
	os.Setenv("DEMO_BAD_PORT", "eighty")
	os.Unsetenv("DEMO_MISSING")

	// Getenv cannot tell "unset" from "set to empty".
	fmt.Printf("Getenv:    missing=%q empty=%q\n", os.Getenv("DEMO_MISSING"), os.Getenv("DEMO_EMPTY"))
	_, okMissing := os.LookupEnv("DEMO_MISSING")
	_, okEmpty := os.LookupEnv("DEMO_EMPTY")
	fmt.Println("LookupEnv: missing set =", okMissing, "| empty set =", okEmpty)

	port, err := intVar(os.LookupEnv, "DEMO_PORT", 8080)
	fmt.Println("port:", port, err)
	port, err = intVar(os.LookupEnv, "DEMO_MISSING", 8080)
	fmt.Println("default port:", port, err)
	_, err = intVar(os.LookupEnv, "DEMO_BAD_PORT", 8080)
	fmt.Println("bad port:", err)

	d, err := durationVar(os.LookupEnv, "DEMO_TIMEOUT", 5*time.Second)
	fmt.Println("timeout:", d, err)

	// The same code against a fake environment: no os.Setenv, no global state.
	fake := func(k string) (string, bool) { v, ok := map[string]string{"DEMO_PORT": "1"}[k]; return v, ok }
	port, _ = intVar(fake, "DEMO_PORT", 8080)
	fmt.Println("fake env port:", port)

	// os.ExpandEnv substitutes ${VAR} in strings (handy for file values).
	fmt.Println(os.Expand("listen on :${DEMO_PORT}", os.Getenv))
}
