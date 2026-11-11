// Errors as values: sentinels, custom types, wrapping, errors.Is/As/Join, defer, recover.
package main

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrNotFound is a sentinel: callers compare with errors.Is.
var ErrNotFound = errors.New("not found")

// FieldError is a custom error type: callers extract it with errors.As.
type FieldError struct {
	Field, Msg string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Msg }

func find(id int) error {
	if id != 1 {
		return fmt.Errorf("find task %d: %w", id, ErrNotFound) // %w keeps the chain
	}
	return nil
}

func parsePriority(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("priority %q: %w", s, err)
	}
	if n < 1 || n > 5 {
		return 0, &FieldError{"priority", "must be 1-5"}
	}
	return n, nil
}

// deferred calls run last-in first-out when the function returns.
func deferOrder() {
	for i := range 3 {
		defer fmt.Println("deferred", i)
	}
	fmt.Println("body done")
}

// A named result lets a deferred function change what the caller sees.
func safeDivide(a, b int) (res int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	return a / b, nil
}

func main() {
	err := find(2)
	fmt.Println(err)
	fmt.Println("is not found:", errors.Is(err, ErrNotFound), "| unwrapped:", errors.Unwrap(err) == ErrNotFound)

	_, err = parsePriority("9")
	var fe *FieldError
	if errors.As(err, &fe) {
		fmt.Println("field error on", fe.Field)
	}

	_, err = parsePriority("high")
	var numErr *strconv.NumError
	fmt.Println(err, "| as NumError:", errors.As(err, &numErr))

	joined := errors.Join(&FieldError{"title", "required"}, &FieldError{"tags", "too many"})
	fmt.Println(joined)

	deferOrder()

	fmt.Println(safeDivide(10, 2))
	fmt.Println(safeDivide(1, 0))
}
