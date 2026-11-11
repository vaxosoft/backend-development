package shortener

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		info := &reqInfo{RequestID: r.Header.Get("X-Request-ID")}
		if info.RequestID == "" || len(info.RequestID) > 64 {
			info.RequestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", info.RequestID)

		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r.WithContext(withInfo(r.Context(), info)))

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		attrs := []slog.Attr{
			slog.String("request_id", info.RequestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
		}
		if info.UserID != 0 {
			attrs = append(attrs, slog.Int64("user_id", info.UserID))
		}
		s.log.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			unauthorized(w)
			return
		}
		userID, err := s.parseToken(token)
		if err != nil {
			unauthorized(w)
			return
		}
		infoFrom(r.Context()).UserID = userID
		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
}

func (s *Server) limitCreates(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := infoFrom(r.Context()).UserID
		limiter, _ := s.limits.LoadOrStore(userID,
			rate.NewLimiter(rate.Limit(s.cfg.RateRefillPerSecond), s.cfg.RateBurst))

		// Reserve tells us how long to wait; we refuse instead of waiting.
		res := limiter.(*rate.Limiter).Reserve()
		if wait := res.Delay(); wait > 0 {
			res.Cancel() // give the token back, a rejected request must not consume one
			retry := max(1, int(math.Ceil(wait.Seconds())))
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many links created, retry later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
