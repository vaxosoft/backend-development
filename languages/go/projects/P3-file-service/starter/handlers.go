package starter

import (
	"encoding/json"
	"log"
	"net/http"
)

// API holds the dependencies of the HTTP handlers.
type API struct {
	store   *Storage
	maxSize int64
}

// multipartOverhead is the room above maxSize that the request body may use
// for multipart boundaries and headers.
const multipartOverhead = 64 << 10

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

// upload handles POST /files: multipart field "file", streamed to storage.
//
// Answer 201 + Location: /files/{id} + Meta JSON; 400 bad_request when the body
// is not multipart or has no "file" part; 413 payload_too_large when the file
// exceeds maxSize (Storage.Save reports ErrTooLarge, and reading the body can
// fail with *http.MaxBytesError); 500 for everything else.
func (a *API) upload(w http.ResponseWriter, r *http.Request) {
	// TODO: http.MaxBytesReader(w, r.Body, a.maxSize+multipartOverhead); r.MultipartReader()
	// (streams, unlike ParseMultipartForm); skip parts until FormName() == "file";
	// SanitizeFilename(part.FileName()); content type from the part header, default
	// application/octet-stream; a.store.Save(part, ...).
	writeError(w, http.StatusNotImplemented, "todo", "upload not implemented")
}

// download handles GET /files/{id}: stream the bytes with Content-Length,
// Content-Type, Content-Disposition: attachment; filename="<name>" and
// ETag: "<sha256>". A matching If-None-Match (or "*") yields 304 with no body.
func (a *API) download(w http.ResponseWriter, r *http.Request) {
	// TODO
	writeError(w, http.StatusNotImplemented, "todo", "download not implemented")
}

// remove handles DELETE /files/{id}: 204, or 404 for an unknown id.
func (a *API) remove(w http.ResponseWriter, r *http.Request) {
	// TODO
	writeError(w, http.StatusNotImplemented, "todo", "delete not implemented")
}
