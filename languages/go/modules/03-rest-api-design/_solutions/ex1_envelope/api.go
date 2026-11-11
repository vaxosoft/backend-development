// Package api writes the JSON responses of a REST API with one error envelope.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
)

var (
	// ErrNotFound is returned (possibly wrapped) when a resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrBadRequest is returned (possibly wrapped) for unusable input such as a bad query value.
	ErrBadRequest = errors.New("bad request")
)

// ValidationError carries one problem per field.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	names := make([]string, 0, len(e.Fields))
	for name := range e.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return "validation failed: " + strings.Join(names, ", ")
}

// WriteJSON sends v as JSON with the given status. The Content-Type header
// must be application/json and must be set before the status is written.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// WriteError sends the error envelope:
//
//	{"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"..."}}}
//
// "fields" is left out entirely when it is empty.
func WriteError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	WriteJSON(w, status, map[string]errorBody{"error": {Code: code, Message: message, Fields: fields}})
}

// WriteErr translates an error into an envelope using errors.Is / errors.As,
// so wrapped errors work too:
//
//	*ValidationError            -> 422 validation_failed, message "invalid request", fields from the error
//	ErrNotFound                 -> 404 not_found, message "resource not found"
//	ErrBadRequest               -> 400 bad_request, message err.Error()
//	*http.MaxBytesError         -> 413 payload_too_large, message "request body too large"
//	anything else               -> 500 internal, message "internal server error" (never the error text)
func WriteErr(w http.ResponseWriter, err error) {
	var (
		verr   *ValidationError
		tooBig *http.MaxBytesError
	)
	switch {
	case errors.As(err, &verr):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid request", verr.Fields)
	case errors.Is(err, ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "resource not found", nil)
	case errors.Is(err, ErrBadRequest):
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
	case errors.As(err, &tooBig):
		WriteError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large", nil)
	default:
		WriteError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
	}
}
