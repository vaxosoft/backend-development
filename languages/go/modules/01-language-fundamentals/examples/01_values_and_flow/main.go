// Types, zero values, constants, control flow, functions and closures.
package main

import (
	"fmt"
	"strings"
)

// Status is a defined type with iota constants: Go's idiom for enums.
type Status int

const (
	Todo Status = iota
	InProgress
	Done
)

func (s Status) String() string {
	switch s {
	case Todo:
		return "todo"
	case InProgress:
		return "in_progress"
	case Done:
		return "done"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// divmod returns two values; callers receive both or name them.
func divmod(a, b int) (quo, rem int) {
	return a / b, a % b
}

// counter returns a closure that owns its own n.
func counter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

func main() {
	// Every type has a zero value; there is no "uninitialised" variable.
	var (
		n int
		s string
		b bool
		f float64
		p *int
		m map[string]int
		l []string
	)
	fmt.Printf("zero values: %d %q %t %.1f %v %v %v\n", n, s, b, f, p, m == nil, l == nil)

	// := declares and infers; var is for zero values or package scope.
	title := "  Write docs  "
	fmt.Printf("trimmed: %q\n", strings.TrimSpace(title))

	var st Status
	fmt.Println("default status:", st, "| constant:", Done, "| unknown:", Status(7))

	q, r := divmod(17, 5)
	fmt.Println("divmod:", q, r)

	// switch needs no break and can have no tag.
	for _, days := range []int{-2, 0, 5} {
		switch {
		case days < 0:
			fmt.Println(days, "-> overdue")
		case days == 0:
			fmt.Println(days, "-> due today")
		default:
			fmt.Println(days, "-> upcoming")
		}
	}

	// for is the only loop. range over an int (Go 1.22+) and over a slice.
	for i := range 3 {
		fmt.Print(i, " ")
	}
	fmt.Println()
	for i, w := range strings.Fields("a b c") {
		fmt.Print(i, "=", w, " ")
	}
	fmt.Println()

	// Closures capture variables, not values. Since Go 1.22 each iteration
	// has its own loop variable, so this prints 0 1 2 (it printed 3 3 3 before).
	var funcs []func() int
	for i := range 3 {
		funcs = append(funcs, func() int { return i })
	}
	for _, fn := range funcs {
		fmt.Print(fn(), " ")
	}
	fmt.Println()

	next := counter()
	next()
	next()
	fmt.Println("counter:", next())
}
