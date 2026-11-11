package aggregator

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseIDs turns the raw `ids` query value ("1,2,3") into integers.
func ParseIDs(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("query parameter ids is required, e.g. ids=1,2,3")
	}
	parts := strings.Split(raw, ",")
	if len(parts) > MaxIDs {
		return nil, fmt.Errorf("at most %d ids are allowed, got %d", MaxIDs, len(parts))
	}
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		id, err := strconv.Atoi(p)
		if err != nil || id < 0 {
			return nil, fmt.Errorf("id %q is not a non-negative integer", p)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
