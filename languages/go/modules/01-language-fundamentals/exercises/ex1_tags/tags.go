// Package tags normalises task tags.
package tags

// Normalize returns the tags trimmed and lowercased, without empty entries
// and without duplicates, keeping the first occurrence of each tag in order.
//
// The input slice must not be modified, and the result is never nil (so it
// encodes as [] rather than null in JSON).
func Normalize(in []string) []string {
	panic("TODO")
}

// Window returns at most n tags from the start of tags. Appending to the
// result must never overwrite elements of the original slice.
func Window(tags []string, n int) []string {
	panic("TODO")
}
