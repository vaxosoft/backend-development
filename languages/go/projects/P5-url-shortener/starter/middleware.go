package shortener

import "net/http"

// logRequests wraps the whole mux.
// TODO:
//   - take the request id from X-Request-ID (when present, at most 64 bytes) or
//     generate one with newRequestID, set it on the response header and
//     store a *reqInfo in the request context (withInfo);
//   - after the handler returns, write ONE JSON line with s.log:
//     msg "request", request_id, method, path, status, duration_ms and
//     user_id (only when info.UserID != 0). Use a statusRecorder to learn the
//     status; 0 means 200;
//   - never log the Authorization header, bodies or query strings.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return next
}

// requireAuth rejects requests without a valid "Authorization: Bearer <jwt>".
// TODO: answer 401 "unauthorized" (envelope) otherwise; on success store the
// user id in infoFrom(r.Context()).UserID so handlers and the logger see it.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return next
}

// limitCreates is a token bucket per authenticated user.
// TODO: look up (or create) a *rate.Limiter for the user in s.limits
// (golang.org/x/time/rate, burst s.cfg.RateBurst, refill
// s.cfg.RateRefillPerSecond). When no token is available answer 429
// "rate_limited" with an integer Retry-After header (seconds, at least 1).
func (s *Server) limitCreates(next http.Handler) http.Handler {
	return next
}
