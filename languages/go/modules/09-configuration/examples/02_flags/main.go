// Example 02: the flag package with a private FlagSet, a custom flag.Value and "was it set?" detection.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// stringList lets a flag be repeated: -tag a -tag b.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

type options struct {
	addr    string
	timeout time.Duration
	verbose bool
	tags    stringList
}

func parse(args []string, out io.Writer) (options, map[string]bool, error) {
	// A private FlagSet (not the global flag.CommandLine) is testable and has no side effects.
	fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
	fs.SetOutput(out) // errors and usage go here, not to the process stderr

	var o options
	fs.StringVar(&o.addr, "addr", "127.0.0.1:8080", "listen `address`")
	fs.DurationVar(&o.timeout, "timeout", 5*time.Second, "request timeout")
	fs.BoolVar(&o.verbose, "verbose", false, "verbose logging")
	fs.Var(&o.tags, "tag", "tag to include (repeatable)")

	if err := fs.Parse(args); err != nil {
		return options{}, nil, err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true }) // only flags that appeared on the command line
	return o, set, nil
}

func main() {
	o, set, err := parse([]string{"-addr", ":9000", "-verbose", "-tag", "docs", "-tag=p1", "extra"}, io.Discard)
	fmt.Println("parsed:", o.addr, o.timeout, o.verbose, o.tags, "err:", err)
	fmt.Println("addr set:", set["addr"], "| timeout set:", set["timeout"])

	_, _, err = parse([]string{"-timeout", "soon"}, io.Discard)
	fmt.Println("error:", err)

	fmt.Println("--- usage for -h ---")
	_, _, err = parse([]string{"-h"}, os.Stdout)
	fmt.Println("help error:", err)
}
