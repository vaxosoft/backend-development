package aggregator

// ParseIDs turns the raw `ids` query value ("1,2,3") into integers.
//
// TODO: return an error (the handler answers 400) when the value is empty,
// has an empty entry ("1,,2"), has a non-numeric or negative entry, or has
// more than MaxIDs entries. Spaces around an entry are allowed ("1, 2").
// Order and duplicates are preserved.
func ParseIDs(raw string) ([]int, error) {
	return nil, nil
}
