package starter

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
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
	r.Body = http.MaxBytesReader(w, r.Body, a.maxSize+multipartOverhead)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "expected a multipart/form-data body")
		return
	}

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "bad_request", `multipart field "file" is required`)
			return
		}
		if err != nil {
			a.uploadFailed(w, err)
			return
		}
		if part.FormName() != "file" {
			continue
		}

		contentType := part.Header.Get("Content-Type")
		if _, _, err := mime.ParseMediaType(contentType); err != nil {
			contentType = "application/octet-stream"
		}
		meta, err := a.store.Save(part, SanitizeFilename(part.FileName()), contentType, a.maxSize)
		if err != nil {
			a.uploadFailed(w, err)
			return
		}
		w.Header().Set("Location", "/files/"+meta.ID)
		writeJSON(w, http.StatusCreated, meta)
		return
	}
}

func (a *API) uploadFailed(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, ErrTooLarge), errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file exceeds the maximum upload size")
	default:
		log.Printf("upload failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

// download handles GET /files/{id}: stream the bytes with Content-Length,
// Content-Type, Content-Disposition: attachment; filename="<name>" and
// ETag: "<sha256>". A matching If-None-Match (or "*") yields 304 with no body.
func (a *API) download(w http.ResponseWriter, r *http.Request) {
	f, meta, err := a.store.Open(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "file not found")
		return
	}
	if err != nil {
		log.Printf("open file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	defer f.Close()

	etag := `"` + meta.SHA256 + `"`
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	w.Header().Set("Content-Disposition", `attachment; filename="`+meta.Name+`"`)
	// Headers are already sent: all that is left to do on failure is to log.
	if _, err := io.Copy(w, f); err != nil {
		log.Printf("stream %s: %v", meta.ID, err)
	}
}

// etagMatches implements the weak comparison of If-None-Match.
func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

// remove handles DELETE /files/{id}: 204, or 404 for an unknown id.
func (a *API) remove(w http.ResponseWriter, r *http.Request) {
	switch err := a.store.Delete(r.PathValue("id")); {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "file not found")
	case err != nil:
		log.Printf("delete file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
