// Example 02: a correlation (request) ID carried in the context and added to every log line automatically.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
)

type ctxKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// contextHandler decorates any slog.Handler: when the context has a request ID, add it as an attribute.
type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

// The wrapper must re-wrap the handlers returned by With*, otherwise decoration is lost after logger.With(...).
func (h contextHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(a)}
}
func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}

// requestID reuses the caller's X-Request-ID or generates one, and echoes it back.
func requestID(next http.Handler, newID func() string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

func main() {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	})
	logger := slog.New(contextHandler{base})

	seq := 0
	newID := func() string { seq++; return fmt.Sprintf("gen-%d", seq) }

	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use the *Context variants so the handler can see the request-scoped values.
		logger.InfoContext(r.Context(), "loading task", "path", r.URL.Path)
		helper(r.Context(), logger)
		fmt.Fprintln(w, "ok")
	})
	handler := requestID(app, newID)

	for _, id := range []string{"client-abc", ""} {
		req := httptest.NewRequest("GET", "/tasks/1", nil)
		if id != "" {
			req.Header.Set("X-Request-ID", id)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		fmt.Println("response header X-Request-ID:", rec.Header().Get("X-Request-ID"))
	}

	// Without the context the ID is missing: the classic mistake of calling logger.Info in a helper.
	logger.Info("no context, no request_id")
}

// helper is deep in the call stack and only needs the context: no logger parameter juggling for the ID.
func helper(ctx context.Context, logger *slog.Logger) {
	logger.With("component", "store").InfoContext(ctx, "query executed", "rows", 1)
}
