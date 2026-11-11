package starter

import (
	"encoding/json"
	"log"
	"net/http"
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
	// TODO: http.MaxBytesReader + json.NewDecoder(...).Decode; map *http.MaxBytesError to 413.
	writeError(w, http.StatusNotImplemented, "todo", "decodeBody not implemented", nil)
	return false
}

// taskID parses the {id} path value. A non-numeric or non-positive id can never
// exist, so callers answer 404 when ok is false.
func taskID(r *http.Request) (id int64, ok bool) {
	// TODO
	return 0, false
}

// parseListQuery validates ?status=&tag=&limit=&offset=&sort=. It applies the
// defaults (limit 20, offset 0, sort -created_at), clamps limit to 100 and
// returns an error (-> 400) for an unknown status or sort, a non-integer
// limit/offset, limit < 1 or offset < 0.
func parseListQuery(r *http.Request) (ListQuery, error) {
	// TODO
	return ListQuery{}, nil
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	// TODO: decode TaskInput, Validate (422 with fields), store.Create, 201 + Location: /tasks/{id}.
	writeError(w, http.StatusNotImplemented, "todo", "create not implemented", nil)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	// TODO: 200 with the task, 404 not_found for ErrNotFound.
	writeError(w, http.StatusNotImplemented, "todo", "get not implemented", nil)
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	// TODO: parseListQuery (400), store.List, listResponse with Items never null.
	writeError(w, http.StatusNotImplemented, "todo", "list not implemented", nil)
}

func (a *API) replace(w http.ResponseWriter, r *http.Request) {
	// TODO: PUT = full replace: absent optional fields reset to their defaults.
	writeError(w, http.StatusNotImplemented, "todo", "replace not implemented", nil)
}

func (a *API) patch(w http.ResponseWriter, r *http.Request) {
	// TODO: only title, status, due_date and tags may change; an absent key is
	// unchanged and "due_date": null clears the date. Decode into
	// map[string]json.RawMessage to tell "absent" from "null".
	writeError(w, http.StatusNotImplemented, "todo", "patch not implemented", nil)
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	// TODO: 204 on success, 404 not_found otherwise.
	writeError(w, http.StatusNotImplemented, "todo", "delete not implemented", nil)
}
