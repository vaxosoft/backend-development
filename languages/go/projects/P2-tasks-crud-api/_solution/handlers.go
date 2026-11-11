package starter

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// API holds the dependencies of the HTTP handlers.
type API struct {
	store Store
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, map[string]errorBody{"error": {Code: code, Message: message, Fields: fields}})
}

// internalError logs the real error and tells the client nothing about it.
func internalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
}

type listResponse struct {
	Items  []Task `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// maxBodyBytes caps every request body.
const maxBodyBytes = 1 << 20

// decodeBody reads one JSON value from the body into dst. On failure it has
// already written the response (400 bad_request, or 413 payload_too_large
// when the body exceeds maxBodyBytes) and returns false.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	err := json.NewDecoder(r.Body).Decode(dst)
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large", nil)
	} else {
		writeError(w, http.StatusBadRequest, "bad_request", "request body must be valid JSON of the expected shape", nil)
	}
	return false
}

// taskID parses the {id} path value. A non-numeric or non-positive id can never
// exist, so callers answer 404 when ok is false.
func taskID(r *http.Request) (id int64, ok bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// parseListQuery validates ?status=&tag=&limit=&offset=&sort=. It applies the
// defaults (limit 20, offset 0, sort -created_at), clamps limit to 100 and
// returns an error (-> 400) for an unknown status or sort, a non-integer
// limit/offset, limit < 1 or offset < 0.
func parseListQuery(r *http.Request) (ListQuery, error) {
	v := r.URL.Query()
	q := ListQuery{Status: v.Get("status"), Tag: v.Get("tag"), Sort: "-created_at", Limit: 20}

	switch q.Status {
	case "", StatusTodo, StatusInProgress, StatusDone:
	default:
		return q, errors.New("status must be one of todo, in_progress, done")
	}
	if s := v.Get("sort"); s != "" {
		if _, ok := orderBy[s]; !ok {
			return q, errors.New("sort must be one of created_at, -created_at, due_date, -due_date")
		}
		q.Sort = s
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return q, errors.New("limit must be a positive integer")
		}
		q.Limit = min(n, 100)
	}
	if s := v.Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return q, errors.New("offset must be a non-negative integer")
		}
		q.Offset = n
	}
	return q, nil
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var in TaskInput
	if !decodeBody(w, r, &in) {
		return
	}
	if problems := in.Validate(); len(problems) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", problems)
		return
	}
	task, err := a.store.Create(r.Context(), in)
	if err != nil {
		internalError(w, err)
		return
	}
	w.Header().Set("Location", "/tasks/"+strconv.FormatInt(task.ID, 10))
	writeJSON(w, http.StatusCreated, task)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		notFound(w)
		return
	}
	task, err := a.store.Get(r.Context(), id)
	a.respond(w, http.StatusOK, task, err)
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	items, total, err := a.store.List(r.Context(), q)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Total: total, Limit: q.Limit, Offset: q.Offset})
}

func (a *API) replace(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		notFound(w)
		return
	}
	var in TaskInput
	if !decodeBody(w, r, &in) {
		return
	}
	if problems := in.Validate(); len(problems) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", problems)
		return
	}
	task, err := a.store.Replace(r.Context(), id, in)
	a.respond(w, http.StatusOK, task, err)
}

func (a *API) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		notFound(w)
		return
	}
	var fields map[string]json.RawMessage
	if !decodeBody(w, r, &fields) {
		return
	}
	current, err := a.store.Get(r.Context(), id)
	if err != nil {
		a.respond(w, http.StatusOK, current, err)
		return
	}

	// Start from the stored values and overwrite only the keys that are present.
	in := TaskInput{Title: current.Title, Status: current.Status, DueDate: current.DueDate, Tags: current.Tags, OwnerID: current.OwnerID}
	targets := map[string]any{"title": &in.Title, "status": &in.Status, "due_date": &in.DueDate, "tags": &in.Tags}
	for key, dst := range targets {
		raw, present := fields[key]
		if !present {
			continue
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("%s has the wrong type", key), nil)
			return
		}
	}

	if problems := in.Validate(); len(problems) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task", problems)
		return
	}
	task, err := a.store.Replace(r.Context(), id, in)
	a.respond(w, http.StatusOK, task, err)
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(r)
	if !ok {
		notFound(w)
		return
	}
	switch err := a.store.Delete(r.Context(), id); {
	case errors.Is(err, ErrNotFound):
		notFound(w)
	case err != nil:
		internalError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found", "task not found", nil)
}

// respond writes task with status, or maps err to 404 / 500.
func (a *API) respond(w http.ResponseWriter, status int, task Task, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		notFound(w)
	case err != nil:
		internalError(w, err)
	default:
		writeJSON(w, status, task)
	}
}
