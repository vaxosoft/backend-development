// Methods, pointer vs value receivers, implicit interfaces, embedding, generics.
package main

import (
	"fmt"
	"strings"
)

type Task struct {
	Title string
	Done  bool
}

// Value receiver: works on a copy, so it cannot change the caller's task.
func (t Task) Label() string {
	if t.Done {
		return "[x] " + t.Title
	}
	return "[ ] " + t.Title
}

// Pointer receiver: mutates the caller's task.
func (t *Task) Complete() { t.Done = true }

// Notifier is satisfied implicitly: no "implements" keyword. Interfaces are
// usually declared by the consumer and kept small.
type Notifier interface {
	Notify(msg string) string
}

type EmailNotifier struct{ Address string }

func (e EmailNotifier) Notify(msg string) string { return "email to " + e.Address + ": " + msg }

type LogNotifier struct{}

func (LogNotifier) Notify(msg string) string { return "log: " + msg }

// Embedding is composition with promotion, not inheritance.
type AuditedNotifier struct {
	Notifier
	sent int
}

func (a *AuditedNotifier) Notify(msg string) string {
	a.sent++
	return fmt.Sprintf("#%d %s", a.sent, a.Notifier.Notify(msg))
}

// Number is a type-set constraint for generics.
type Number interface {
	~int | ~int64 | ~float64
}

func Sum[T Number](xs []T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

// Map transforms a slice; the compiler infers T and U.
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

type myErr struct{}

func (*myErr) Error() string { return "myErr" }

// mayFail returns a typed nil pointer wrapped in a non-nil error interface.
func mayFail() error {
	var e *myErr
	return e
}

func main() {
	t := Task{Title: "Write docs"}
	t.Complete() // the compiler takes &t for us
	fmt.Println(t.Label())

	tasks := []Task{{Title: "a"}, {Title: "b"}}
	for _, task := range tasks {
		task.Complete() // modifies the loop copy only
	}
	fmt.Println("after range copy:", tasks[0].Done, tasks[1].Done)
	for i := range tasks {
		tasks[i].Complete()
	}
	fmt.Println("after index:", tasks[0].Done, tasks[1].Done)

	var n Notifier = EmailNotifier{"ann@example.com"}
	fmt.Println(n.Notify("hi"))
	n = LogNotifier{}
	fmt.Println(n.Notify("hi"))

	audited := &AuditedNotifier{Notifier: LogNotifier{}}
	audited.Notify("one")
	fmt.Println(audited.Notify("two"))

	// Type switch recovers the concrete type behind an interface.
	switch v := n.(type) {
	case EmailNotifier:
		fmt.Println("email", v.Address)
	case LogNotifier:
		fmt.Println("a log notifier")
	}

	fmt.Println(Sum([]int{1, 2, 3}), Sum([]float64{0.5, 0.25}))
	fmt.Println(strings.Join(Map([]int{1, 2, 3}, func(i int) string { return fmt.Sprint(i * i) }), ","))

	// The classic trap: an interface is nil only if BOTH its type and value are nil.
	err := mayFail()
	fmt.Println("err == nil?", err == nil)
}
