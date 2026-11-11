// Package shortener is a URL shortener with registration, JWT login,
// per-user rate limiting and structured request logs.
package shortener

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Config holds everything the server needs. Tests build one directly.
type Config struct {
	DBPath              string        // SQLite file, or ":memory:"
	JWTSecret           []byte        // HS256 signing key
	TokenTTL            time.Duration // lifetime of a login token
	BcryptCost          int           // 0 means bcrypt.DefaultCost; tests use bcrypt.MinCost
	RateBurst           int           // token-bucket size for POST /links
	RateRefillPerSecond float64       // tokens added per second
	LogWriter           io.Writer     // destination of the JSON request log
}

// DefaultConfig returns sensible defaults; the caller supplies the secret.
func DefaultConfig(secret []byte) Config {
	return Config{
		DBPath:              "shortener.db",
		JWTSecret:           secret,
		TokenTTL:            time.Hour,
		BcryptCost:          bcrypt.DefaultCost,
		RateBurst:           10,
		RateRefillPerSecond: 1,
		LogWriter:           os.Stdout,
	}
}

// Server implements http.Handler.
type Server struct {
	cfg       Config
	store     *Store
	log       *slog.Logger
	handler   http.Handler
	dummyHash string   // compared against when the e-mail is unknown, to keep login timing equal
	limits    sync.Map // user id (int64) -> *rate.Limiter
}

// New opens the database, creates the schema and wires the routes.
// Call Close when done.
func New(cfg Config) (*Server, error) {
	if len(cfg.JWTSecret) == 0 {
		return nil, errors.New("shortener: JWTSecret is required")
	}
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = time.Hour
	}
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = bcrypt.DefaultCost
	}
	if cfg.LogWriter == nil {
		cfg.LogWriter = io.Discard
	}

	store, err := openStore(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("not-a-real-password"), cfg.BcryptCost)
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	s := &Server{
		cfg:       cfg,
		store:     store,
		log:       slog.New(slog.NewJSONHandler(cfg.LogWriter, nil)),
		dummyHash: string(dummy),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.Handle("POST /links", s.requireAuth(s.limitCreates(http.HandlerFunc(s.handleCreateLink))))
	mux.Handle("GET /links", s.requireAuth(http.HandlerFunc(s.handleListLinks)))
	mux.Handle("DELETE /links/{slug}", s.requireAuth(http.HandlerFunc(s.handleDeleteLink)))
	mux.HandleFunc("GET /{slug}", s.handleRedirect)
	s.handler = s.logRequests(mux)
	return s, nil
}

// ServeHTTP lets a *Server be used wherever an http.Handler is expected.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// Close releases the database.
func (s *Server) Close() error { return s.store.Close() }
