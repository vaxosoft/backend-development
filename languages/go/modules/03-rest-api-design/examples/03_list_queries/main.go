// Collections: filtering, sorting and offset pagination with a stable envelope.
// Runs in-process with httptest, so no port is needed.
package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
)

type Task struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	DueDate   *string  `json:"due_date"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
}

type Page struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

const (
	defaultLimit = 20
	maxLimit     = 100
)

type query struct {
	status, tag, sort string
	limit, offset     int
}

// parseQuery applies defaults, clamps limit and rejects bad values (-> 400).
func parseQuery(v url.Values) (query, string) {
	q := query{status: v.Get("status"), tag: v.Get("tag"), sort: "-created_at", limit: defaultLimit}
	switch q.status {
	case "", "todo", "in_progress", "done":
	default:
		return q, "status must be todo, in_progress or done"
	}
	if s := v.Get("sort"); s != "" {
		if !slices.Contains([]string{"created_at", "-created_at", "due_date", "-due_date"}, s) {
			return q, "sort must be created_at, -created_at, due_date or -due_date"
		}
		q.sort = s
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return q, "limit must be a positive integer"
		}
		q.limit = min(n, maxLimit) // clamp instead of failing
	}
	if s := v.Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return q, "offset must be a non-negative integer"
		}
		q.offset = n
	}
	return q, ""
}

func list(all []Task, q query) Page {
	var matched []Task
	for _, t := range all {
		if (q.status == "" || t.Status == q.status) && (q.tag == "" || slices.Contains(t.Tags, q.tag)) {
			matched = append(matched, t)
		}
	}
	desc := q.sort[0] == '-'
	slices.SortStableFunc(matched, func(a, b Task) int {
		var c int
		switch q.sort {
		case "created_at", "-created_at":
			c = cmp.Compare(a.CreatedAt, b.CreatedAt)
		default: // tasks without a due date always sort last
			switch {
			case a.DueDate == nil && b.DueDate == nil:
			case a.DueDate == nil:
				return 1
			case b.DueDate == nil:
				return -1
			default:
				c = cmp.Compare(*a.DueDate, *b.DueDate)
			}
		}
		if desc {
			c = -c
		}
		return cmp.Or(c, cmp.Compare(a.ID, b.ID)) // ties: id keeps pages stable
	})

	total := len(matched)
	start := min(q.offset, total)
	end := min(start+q.limit, total)
	items := matched[start:end]
	if items == nil {
		items = []Task{} // encode [] rather than null
	}
	return Page{Items: items, Total: total, Limit: q.limit, Offset: q.offset}
}

func handler(all []Task) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, problem := parseQuery(r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		if problem != "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "bad_request", "message": problem}})
			return
		}
		_ = json.NewEncoder(w).Encode(list(all, q))
	}
}

func main() {
	d := func(s string) *string { return &s }
	tasks := []Task{
		{1, "Write docs", "todo", d("2030-03-01"), []string{"docs"}, "2030-01-01T09:00:00Z"},
		{2, "Review PR", "in_progress", d("2030-01-15"), []string{"code", "p1"}, "2030-01-02T09:00:00Z"},
		{3, "Ship release", "done", nil, []string{"p1"}, "2030-01-03T09:00:00Z"},
		{4, "Fix bug", "todo", d("2030-02-01"), []string{"code", "p1"}, "2030-01-04T09:00:00Z"},
		{5, "Plan sprint", "todo", nil, nil, "2030-01-05T09:00:00Z"},
	}
	h := handler(tasks)

	for _, target := range []string{
		"/tasks",
		"/tasks?limit=2",
		"/tasks?limit=2&offset=2",
		"/tasks?limit=2&offset=4",
		"/tasks?offset=50",
		"/tasks?status=todo&sort=due_date",
		"/tasks?status=todo&sort=-due_date",
		"/tasks?tag=p1&sort=created_at",
		"/tasks?limit=1000",
		"/tasks?limit=abc",
		"/tasks?sort=title",
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("GET", target, nil))

		var page Page
		if rec.Code != http.StatusOK {
			fmt.Printf("%-36s %d %s", target, rec.Code, rec.Body.String())
			continue
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &page)
		ids := make([]int, 0, len(page.Items))
		for _, t := range page.Items {
			ids = append(ids, t.ID)
		}
		fmt.Printf("%-36s %d ids=%v total=%d limit=%d offset=%d\n", target, rec.Code, ids, page.Total, page.Limit, page.Offset)
	}
}
