package listing

import (
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"testing"
	"time"
)

func parse(t *testing.T, q string) (Params, error) {
	t.Helper()
	v, err := url.ParseQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	return ParseParams(v)
}

func TestParseParamsDefaults(t *testing.T) {
	p, err := parse(t, "")
	if err != nil {
		t.Fatal(err)
	}
	want := Params{Sort: "-created_at", Limit: 20, Offset: 0}
	if p != want {
		t.Fatalf("got %+v, want %+v", p, want)
	}
}

func TestParseParamsValid(t *testing.T) {
	tests := []struct {
		query string
		want  Params
	}{
		{"limit=5&offset=10", Params{Sort: "-created_at", Limit: 5, Offset: 10}},
		{"limit=100", Params{Sort: "-created_at", Limit: 100}},
		{"limit=101", Params{Sort: "-created_at", Limit: 100}},
		{"limit=100000", Params{Sort: "-created_at", Limit: 100}},
		{"status=done&tag=p1&sort=due_date", Params{Status: "done", Tag: "p1", Sort: "due_date", Limit: 20}},
		{"sort=-due_date", Params{Sort: "-due_date", Limit: 20}},
		{"sort=created_at", Params{Sort: "created_at", Limit: 20}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := parse(t, tt.query)
			if err != nil || got != tt.want {
				t.Fatalf("ParseParams(%q) = %+v, %v; want %+v", tt.query, got, err, tt.want)
			}
		})
	}
}

func TestParseParamsRejectsBadValues(t *testing.T) {
	tests := []struct{ query, param string }{
		{"limit=0", "limit"},
		{"limit=-5", "limit"},
		{"limit=abc", "limit"},
		{"limit=2.5", "limit"},
		{"offset=-1", "offset"},
		{"offset=x", "offset"},
		{"sort=title", "sort"},
		{"sort=-title", "sort"},
		{"status=blocked", "status"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			_, err := parse(t, tt.query)
			var qe *QueryError
			if !errors.As(err, &qe) {
				t.Fatalf("err = %v, want *QueryError", err)
			}
			if qe.Param != tt.param {
				t.Fatalf("Param = %q, want %q", qe.Param, tt.param)
			}
		})
	}
}

func day(n int) time.Time { return time.Date(2030, 1, n, 9, 0, 0, 0, time.UTC) }

func ptr(s string) *string { return &s }

func ids(tasks []Task) []int {
	out := make([]int, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

func TestSort(t *testing.T) {
	base := []Task{
		{ID: 1, DueDate: ptr("2030-03-01"), CreatedAt: day(1)},
		{ID: 2, DueDate: ptr("2030-01-15"), CreatedAt: day(2)},
		{ID: 3, DueDate: nil, CreatedAt: day(3)},
		{ID: 4, DueDate: ptr("2030-01-15"), CreatedAt: day(4)},
		{ID: 5, DueDate: nil, CreatedAt: day(5)},
	}
	tests := []struct {
		sort string
		want []int
	}{
		{"created_at", []int{1, 2, 3, 4, 5}},
		{"-created_at", []int{5, 4, 3, 2, 1}},
		{"due_date", []int{2, 4, 1, 3, 5}},  // ties by id, nulls last
		{"-due_date", []int{1, 2, 4, 3, 5}}, // nulls still last
	}
	for _, tt := range tests {
		t.Run(tt.sort, func(t *testing.T) {
			tasks := slices.Clone(base)
			Sort(tasks, tt.sort)
			if got := ids(tasks); !slices.Equal(got, tt.want) {
				t.Fatalf("order = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaginate(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	tests := []struct {
		name       string
		limit, off int
		want       []int
	}{
		{"first page", 2, 0, []int{1, 2}},
		{"middle", 2, 2, []int{3, 4}},
		{"partial last page", 2, 4, []int{5}},
		{"limit bigger than data", 20, 0, []int{1, 2, 3, 4, 5}},
		{"offset at end", 2, 5, []int{}},
		{"offset past end", 2, 50, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Paginate(items, Params{Limit: tt.limit, Offset: tt.off})
			if got.Items == nil || !slices.Equal(got.Items, tt.want) {
				t.Fatalf("Items = %#v, want %#v", got.Items, tt.want)
			}
			if got.Total != 5 || got.Limit != tt.limit || got.Offset != tt.off {
				t.Fatalf("envelope = %+v", got)
			}
		})
	}
}

func TestPaginateEncodesEmptyPageAsArrayAndDoesNotAliasInput(t *testing.T) {
	b, err := json.Marshal(Paginate([]string{"a"}, Params{Limit: 5, Offset: 9}))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"items":[],"total":1,"limit":5,"offset":9}`; string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}

	items := []int{1, 2, 3}
	page := Paginate(items, Params{Limit: 2})
	page.Items[0] = 99
	if items[0] != 1 {
		t.Fatal("the page shares memory with the input slice")
	}
}
