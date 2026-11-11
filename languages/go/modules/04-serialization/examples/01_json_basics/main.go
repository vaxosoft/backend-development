// Struct tags, omitempty vs omitzero, unexported fields and decoding into any.
package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type Task struct {
	ID       int      `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Tags     []string `json:"tags"`
	Priority int      `json:"priority,omitempty"` // 0 disappears: omitempty cannot tell "unset" from 0
	Done     bool     `json:"done,omitzero"`
	Notes    string   `json:"-"` // never serialized
	secret   string   // unexported: silently skipped
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	return string(b)
}

func main() {
	t := Task{ID: 1, Title: "Write docs", Status: "todo", Notes: "internal", secret: "x"}
	fmt.Println(mustJSON(t))

	// A nil slice encodes as null, an empty one as []. Clients usually want [].
	fmt.Println(mustJSON(Task{Tags: nil}))
	fmt.Println(mustJSON(Task{Tags: []string{}}))

	pretty, err := json.MarshalIndent(Task{ID: 2, Title: "Ship", Tags: []string{"p1"}}, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(pretty))

	// Unknown fields are ignored and matching is case-insensitive by default.
	var back Task
	if err := json.Unmarshal([]byte(`{"ID":7,"TITLE":"Loose","extra":true}`), &back); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", back)

	// Decoding into any: every JSON number becomes float64.
	var generic map[string]any
	if err := json.Unmarshal([]byte(`{"id":9007199254740993,"tags":["a"]}`), &generic); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%T %v\n", generic["id"], generic["id"])

	// Type mismatches are reported with the offending field.
	err = json.Unmarshal([]byte(`{"id":"one"}`), &back)
	fmt.Println(err)
}
