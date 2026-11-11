// Package tags normalises task tags.
package tags

import "strings"

// Normalize returns the tags trimmed and lowercased, without empty entries
// and without duplicates, keeping the first occurrence of each tag in order.
//
// The input slice must not be modified, and the result is never nil (so it
// encodes as [] rather than null in JSON).
func Normalize(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// Window returns at most n tags from the start of tags. Appending to the
// result must never overwrite elements of the original slice.
func Window(tags []string, n int) []string {
	n = max(0, min(n, len(tags)))
	return tags[:n:n] // full slice expression caps the capacity
}
