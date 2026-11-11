package tags

import (
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil input gives empty non-nil", nil, []string{}},
		{"lowercase and trim", []string{" Docs ", "P1"}, []string{"docs", "p1"}},
		{"drops empty", []string{"", "  ", "a"}, []string{"a"}},
		{"dedupes case-insensitively keeping first", []string{"Go", "rust", "GO", "go ", "Rust"}, []string{"go", "rust"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.in)
			if got == nil || !slices.Equal(got, tt.want) {
				t.Fatalf("Normalize(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeDoesNotModifyInput(t *testing.T) {
	in := []string{" A", "a", "B "}
	snapshot := slices.Clone(in)
	Normalize(in)
	if !slices.Equal(in, snapshot) {
		t.Fatalf("input modified: %q", in)
	}
}

func TestWindow(t *testing.T) {
	tags := []string{"a", "b", "c", "d"}
	tests := []struct {
		n    int
		want []string
	}{{2, []string{"a", "b"}}, {0, []string{}}, {10, []string{"a", "b", "c", "d"}}, {-1, []string{}}}
	for _, tt := range tests {
		if got := Window(tags, tt.n); !slices.Equal(got, tt.want) {
			t.Errorf("Window(n=%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestWindowAppendDoesNotClobberOriginal(t *testing.T) {
	tags := []string{"a", "b", "c"}
	w := Window(tags, 1)
	_ = append(w, "X")
	if !slices.Equal(tags, []string{"a", "b", "c"}) {
		t.Fatalf("append through the window overwrote the original: %q", tags)
	}
}
