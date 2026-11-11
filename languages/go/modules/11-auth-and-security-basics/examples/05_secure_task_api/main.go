// Example 05: Task Tracker API with registration, bcrypt, JWT, roles, ownership checks and per-user rate limiting.
//
//	ADDR=127.0.0.1:8080 go run ./modules/11-auth-and-security-basics/examples/05_secure_task_api
//
// Settings (environment): JWT_SECRET (>= 32 bytes), ADMIN_PASSWORD (seeds admin@example.com),
// RATE_BURST and RATE_PER_SEC (POST /tasks limit per user), TLS_CERT and TLS_KEY (serve HTTPS).
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

const (
	roleMember = "member"
	roleAdmin  = "admin"
	issuer     = "tasktracker"
)

type user struct {
	ID    int
	Email string
	Hash  []byte
	Role  string
}

type task struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	OwnerID int    `json:"owner_id"`
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type principal struct {
	ID   int
	Role string
}

type ctxKey struct{}

type api struct {
	secret    []byte
	burst     int
	perSecond rate.Limit
	dummyHash []byte // compared when the email is unknown so both failures cost the same

	mu       sync.Mutex
	users    map[string]*user // by email
	tasks    map[int]*task
	nextUser int
	nextTask int
	limiters map[int]*rate.Limiter
}

func newAPI(secret []byte, adminPassword string, burst int, perSecond float64) (*api, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	a := &api{secret: secret, burst: burst, perSecond: rate.Limit(perSecond), dummyHash: dummy,
		users: map[string]*user{}, tasks: map[int]*task{}, limiters: map[int]*rate.Limiter{}}
	if _, err := a.addUser("admin@example.com", adminPassword, roleAdmin); err != nil {
		return nil, err
	}
	return a, nil
}

var errExists = errors.New("email already registered")

func (a *api) addUser(email, password, role string) (*user, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.users[email]; ok {
		return nil, errExists
	}
	a.nextUser++
	u := &user{ID: a.nextUser, Email: email, Hash: hash, Role: role}
	a.users[email] = u
	return u, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	e := map[string]any{"code": code, "message": msg}
	if fields != nil {
		e["fields"] = fields
	}
	writeJSON(w, status, map[string]any{"error": e})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body", nil)
		return false
	}
	return true
}

func (a *api) issue(u *user) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{Role: u.Role, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: issuer, Subject: strconv.Itoa(u.ID), IssuedAt: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}).SignedString(a.secret)
}

func (a *api) parse(token string) (principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return a.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer(issuer))
	if err != nil {
		return principal{}, err
	}
	id, err := strconv.Atoi(c.Subject)
	if err != nil {
		return principal{}, err
	}
	return principal{ID: id, Role: c.Role}, nil
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid credentials", nil)
}

// authenticate puts the verified principal in the request context.
func (a *api) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			unauthorized(w)
			return
		}
		p, err := a.parse(token)
		if err != nil {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
	})
}

func requireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if me(r).Role != role {
			writeError(w, http.StatusForbidden, "forbidden", "not allowed", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func me(r *http.Request) principal { return r.Context().Value(ctxKey{}).(principal) }

// limit allows burst requests at once, then perSecond per second, tracked per user.
func (a *api) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := me(r).ID
		a.mu.Lock()
		lim, ok := a.limiters[id]
		if !ok {
			lim = rate.NewLimiter(a.perSecond, a.burst)
			a.limiters[id] = lim
		}
		a.mu.Unlock()
		res := lim.Reserve()
		if delay := res.Delay(); delay > 0 {
			res.Cancel() // do not borrow from the future for a rejected request
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(delay.Seconds()))))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *api) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	fields := map[string]string{}
	if !strings.Contains(in.Email, "@") {
		fields["email"] = "must contain @"
	}
	if n := len(in.Password); n < 8 || n > 72 { // bcrypt only reads the first 72 bytes
		fields["password"] = "must be 8-72 bytes"
	}
	if len(fields) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid credentials", fields)
		return
	}
	u, err := a.addUser(strings.ToLower(in.Email), in.Password, roleMember)
	if errors.Is(err, errExists) {
		writeError(w, http.StatusConflict, "conflict", "email already registered", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": u.ID, "email": u.Email})
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	u := a.users[strings.ToLower(in.Email)]
	a.mu.Unlock()
	hash := a.dummyHash
	if u != nil {
		hash = u.Hash
	}
	err := bcrypt.CompareHashAndPassword(hash, []byte(in.Password))
	if u == nil || err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email or password", nil)
		return
	}
	token, err := a.issue(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (a *api) listTasks(w http.ResponseWriter, r *http.Request) {
	p := me(r)
	a.mu.Lock()
	items := []task{}
	for id := 1; id <= a.nextTask; id++ {
		if t, ok := a.tasks[id]; ok && (p.Role == roleAdmin || t.OwnerID == p.ID) {
			items = append(items, *t)
		}
	}
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (a *api) createTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if n := len([]rune(in.Title)); n < 1 || n > 200 {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "invalid task",
			map[string]string{"title": "must be 1-200 characters"})
		return
	}
	a.mu.Lock()
	a.nextTask++
	t := &task{ID: a.nextTask, Title: in.Title, Status: "todo", OwnerID: me(r).ID}
	a.tasks[t.ID] = t
	a.mu.Unlock()
	w.Header().Set("Location", fmt.Sprintf("/tasks/%d", t.ID))
	writeJSON(w, http.StatusCreated, t)
}

func (a *api) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.tasks[id]
	if err != nil || !ok {
		writeError(w, http.StatusNotFound, "not_found", "task not found", nil)
		return
	}
	if p := me(r); t.OwnerID != p.ID && p.Role != roleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "not allowed", nil)
		return
	}
	delete(a.tasks, id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	items := make([]map[string]any, a.nextUser)
	for _, u := range a.users {
		items[u.ID-1] = map[string]any{"id": u.ID, "email": u.Email, "role": u.Role}
	}
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *api) routes() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.Handler { return a.authenticate(h) }
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.login)
	mux.Handle("GET /tasks", auth(a.listTasks))
	mux.Handle("POST /tasks", a.authenticate(a.limit(http.HandlerFunc(a.createTask))))
	mux.Handle("DELETE /tasks/{id}", auth(a.deleteTask))
	mux.Handle("GET /admin/users", a.authenticate(requireRole(roleAdmin, http.HandlerFunc(a.listUsers))))
	return securityHeaders(mux)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	burst, _ := strconv.Atoi(env("RATE_BURST", "3"))
	perSecond, _ := strconv.ParseFloat(env("RATE_PER_SEC", "1"), 64)
	// The defaults are for local demos only: real secrets come from the environment or a secret store.
	a, err := newAPI([]byte(env("JWT_SECRET", "demo-secret-at-least-32-bytes-long!")),
		env("ADMIN_PASSWORD", "admin-demo-pass"), burst, perSecond)
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Addr:              env("ADDR", "127.0.0.1:8080"),
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		if cert, key := os.Getenv("TLS_CERT"), os.Getenv("TLS_KEY"); cert != "" && key != "" {
			log.Printf("listening on https://%s", srv.Addr)
			errc <- srv.ListenAndServeTLS(cert, key)
			return
		}
		log.Printf("listening on http://%s", srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		log.Fatal(err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Print("shutdown: ", err)
	}
	log.Print("stopped")
}
