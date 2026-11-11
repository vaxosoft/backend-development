// Slices (length, capacity, aliasing), maps and strings.
package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type person struct {
	Name string
	Age  int
}

func main() {
	// A slice is a (pointer, len, cap) view of an array.
	a := make([]int, 3, 4)
	b := append(a, 99) // fits in cap: shares a's backing array
	c := append(a, 100)
	fmt.Println("shared backing array:", b[3], c[3]) // both 100: b was overwritten

	d := append(b, 5) // exceeds cap: copies to a new array
	d[0] = -1
	fmt.Println("after realloc:", a[0], d[0])

	// Slicing does not copy either.
	nums := []int{1, 2, 3, 4, 5}
	head := nums[:2]
	head = append(head, 42) // overwrites nums[2]
	fmt.Println("nums after append on sub-slice:", nums)

	// Use slices.Clone (or a full slice expression nums[:2:2]) to avoid it.
	safe := append(slices.Clone(nums[:2]), 7)
	fmt.Println("clone leaves nums alone:", nums, safe)

	// A nil slice is a valid empty slice; append works on it.
	var tags []string
	tags = append(tags, "docs", "p1")
	fmt.Println("len/cap:", len(tags), cap(tags) >= 2)

	// Maps: reads of missing keys give the zero value; use comma-ok to tell.
	counts := map[string]int{}
	for _, w := range strings.Fields("go is fun and go is fast") {
		counts[w]++
	}
	v, ok := counts["rust"]
	fmt.Println("missing key:", v, ok)

	// Iteration order of a map is random on purpose: sort keys to be deterministic.
	for _, k := range slices.Sorted(maps.Keys(counts)) {
		fmt.Printf("%s=%d ", k, counts[k])
	}
	fmt.Println()

	var nilMap map[string]int
	fmt.Println("read from nil map:", nilMap["x"])
	// nilMap["x"] = 1 would panic: assignment to entry in nil map

	// Strings are immutable UTF-8 bytes; range yields runes.
	word := "héllo"
	fmt.Println("bytes:", len(word), "runes:", len([]rune(word)))
	for i, r := range "hé" {
		fmt.Printf("%d:%c ", i, r)
	}
	fmt.Println()

	var sb strings.Builder
	for i := range 3 {
		fmt.Fprintf(&sb, "[%d]", i)
	}
	fmt.Println(sb.String())

	people := []person{{"Cy", 30}, {"Al", 25}, {"Bo", 30}}
	slices.SortFunc(people, func(x, y person) int {
		if c := y.Age - x.Age; c != 0 {
			return c
		}
		return strings.Compare(x.Name, y.Name)
	})
	fmt.Println(people)
}
