package shortener

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
)

// reqInfo is a mutable per-request record. The logging middleware creates it,
// the auth middleware fills in UserID, and the logger reads it afterwards.
type reqInfo struct {
	RequestID string
	UserID    int64 // 0 while the caller is anonymous
}

type ctxKey struct{}

func withInfo(ctx context.Context, info *reqInfo) context.Context {
	return context.WithValue(ctx, ctxKey{}, info)
}

// infoFrom never returns nil, so handlers can read it unconditionally.
func infoFrom(ctx context.Context) *reqInfo {
	if info, ok := ctx.Value(ctxKey{}).(*reqInfo); ok {
		return info
	}
	return &reqInfo{}
}

// statusRecorder remembers the status code a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: message}})
}

func writeValidation(w http.ResponseWriter, message string, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity,
		map[string]apiError{"error": {Code: "validation_failed", Message: message, Fields: fields}})
}

// internal logs the real error and sends a generic 500 (never leak details).
func (s *Server) internal(w http.ResponseWriter, r *http.Request, err error) {
	s.log.ErrorContext(r.Context(), "internal error", "request_id", infoFrom(r.Context()).RequestID, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal server error")
}

// decodeJSON reads a size-limited JSON body. On failure it has already
// answered 400 and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body")
		return false
	}
	return true
}
