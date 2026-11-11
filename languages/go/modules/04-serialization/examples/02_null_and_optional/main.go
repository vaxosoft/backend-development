// Absent vs null vs value: pointers, and a tri-state Optional[T] for PATCH.
package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// Optional distinguishes a missing key from an explicit null.
// encoding/json only calls UnmarshalJSON when the key is present,
// and calls it for null too, because the field is not a pointer.
type Optional[T any] struct {
	Set   bool // key was present
	Null  bool // value was null
	Value T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

type Patch struct {
	Title   Optional[string] `json:"title"`
	DueDate Optional[string] `json:"due_date"`
}

type TaskView struct {
	Title   string  `json:"title"`
	DueDate *string `json:"due_date"` // nil -> null, always present
}

type TaskOmit struct {
	Title   string  `json:"title"`
	DueDate *string `json:"due_date,omitempty"` // nil -> key dropped
}

func main() {
	due := "2030-01-31"
	b, _ := json.Marshal(TaskView{Title: "a", DueDate: &due})
	fmt.Println(string(b))
	b, _ = json.Marshal(TaskView{Title: "b"})
	fmt.Println(string(b))
	b, _ = json.Marshal(TaskOmit{Title: "c"})
	fmt.Println(string(b))

	// With a plain *string, absent and null both end up nil: indistinguishable.
	for _, body := range []string{`{}`, `{"due_date":null}`, `{"due_date":"2030-02-01"}`} {
		var v TaskView
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("pointer  %-26s nil=%v\n", body, v.DueDate == nil)
	}

	for _, body := range []string{`{}`, `{"due_date":null}`, `{"due_date":"2030-02-01"}`} {
		var p Patch
		if err := json.Unmarshal([]byte(body), &p); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("optional %-26s set=%v null=%v value=%q\n", body, p.DueDate.Set, p.DueDate.Null, p.DueDate.Value)
	}
}
