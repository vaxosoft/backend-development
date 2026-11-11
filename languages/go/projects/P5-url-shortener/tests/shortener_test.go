package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	shortener "webdev/projects/P5-url-shortener/starter"
)

var secret = []byte("test-secret-test-secret-test-1234")

// syncBuffer is a log sink that is safe to read while the server writes.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// lines waits (briefly) until at least n log lines exist and returns them parsed.
func (b *syncBuffer) lines(t *testing.T, n int) []map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var out []map[string]any
		for _, raw := range strings.Split(strings.TrimSpace(b.String()), "\n") {
			if raw == "" {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				t.Fatalf("log line is not JSON: %q", raw)
			}
			out = append(out, m)
		}
		if len(out) >= n || time.Now().After(deadline) {
			if len(out) != n {
				t.Fatalf("got %d log lines, want %d:\n%s", len(out), n, b.String())
			}
			return out
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type env struct {
	t      *testing.T
	url    string
	dbPath string
	logs   *syncBuffer
	client *http.Client
}

func newEnv(t *testing.T, tune func(*shortener.Config)) *env {
	t.Helper()
	e := &env{
		t:      t,
		dbPath: filepath.Join(t.TempDir(), "shortener.db"),
		logs:   &syncBuffer{},
		client: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	cfg := shortener.DefaultConfig(secret)
	cfg.DBPath = e.dbPath
	cfg.BcryptCost = bcrypt.MinCost
	cfg.RateBurst = 100
	cfg.RateRefillPerSecond = 100
	cfg.LogWriter = e.logs
	if tune != nil {
		tune(&cfg)
	}
	app, err := shortener.New(cfg)
	if err != nil {
		t.Fatalf("shortener.New: %v", err)
	}
	srv := httptest.NewServer(app)
	t.Cleanup(func() {
		srv.Close()
		_ = app.Close()
	})
	e.url = srv.URL
	return e
}

func (e *env) do(method, path, token string, body any, headers ...string) (*http.Response, []byte) {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.url+path, rd)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp, data
}

func obj(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, raw)
	}
	return m
}

func expect(t *testing.T, resp *http.Response, body []byte, status int) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, status, body)
	}
}

// expectError checks the shared error envelope and returns its fields map.
func expectError(t *testing.T, resp *http.Response, body []byte, status int, code string) map[string]any {
	t.Helper()
	expect(t, resp, body, status)
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	errObj, _ := obj(t, body)["error"].(map[string]any)
	if errObj == nil || errObj["code"] != code {
		t.Fatalf("body = %s, want error code %q", body, code)
	}
	if msg, _ := errObj["message"].(string); msg == "" {
		t.Errorf("error message is empty: %s", body)
	}
	fields, _ := errObj["fields"].(map[string]any)
	return fields
}

func (e *env) register(email, password string) float64 {
	e.t.Helper()
	resp, body := e.do("POST", "/auth/register", "", map[string]string{"email": email, "password": password})
	expect(e.t, resp, body, http.StatusCreated)
	id, _ := obj(e.t, body)["id"].(float64)
	return id
}

func (e *env) login(email, password string) string {
	e.t.Helper()
	resp, body := e.do("POST", "/auth/login", "", map[string]string{"email": email, "password": password})
	expect(e.t, resp, body, http.StatusOK)
	token, _ := obj(e.t, body)["token"].(string)
	if token == "" {
		e.t.Fatalf("login returned no token: %s", body)
	}
	return token
}

// signup registers a user and returns a token.
func (e *env) signup(email string) string {
	e.t.Helper()
	e.register(email, "correct-horse")
	return e.login(email, "correct-horse")
}

func (e *env) createLink(token string, payload map[string]string) (*http.Response, []byte) {
	e.t.Helper()
	return e.do("POST", "/links", token, payload)
}

func (e *env) mustCreate(token, slug, target string) {
	e.t.Helper()
	resp, body := e.createLink(token, map[string]string{"url": target, "slug": slug})
	expect(e.t, resp, body, http.StatusCreated)
}

// --- registration and login -----------------------------------------------------

func TestRegisterCreatesUser(t *testing.T) {
	e := newEnv(t, nil)
	resp, body := e.do("POST", "/auth/register", "", map[string]string{"email": "ann@example.com", "password": "correct-horse"})
	expect(t, resp, body, http.StatusCreated)
	got := obj(t, body)
	if got["email"] != "ann@example.com" || got["id"] == nil {
		t.Errorf("body = %s, want id and email", body)
	}
	if strings.Contains(string(body), "correct-horse") || strings.Contains(string(body), "$2") {
		t.Errorf("response leaks the password or its hash: %s", body)
	}
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv(t, nil)
	tests := []struct{ name, email, password, field string }{
		{"email without @", "no-at-sign", "correct-horse", "email"},
		{"empty email", "", "correct-horse", "email"},
		{"password too short", "a@example.com", "short7!", "password"},
		{"password beyond bcrypt's 72 bytes", "a@example.com", strings.Repeat("x", 73), "password"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := e.do("POST", "/auth/register", "", map[string]string{"email": tc.email, "password": tc.password})
			fields := expectError(t, resp, body, http.StatusUnprocessableEntity, "validation_failed")
			if _, ok := fields[tc.field]; !ok {
				t.Errorf("fields = %v, want an entry for %q", fields, tc.field)
			}
		})
	}
}

func TestRegisterDuplicateEmailIs409(t *testing.T) {
	e := newEnv(t, nil)
	e.register("dup@example.com", "correct-horse")
	resp, body := e.do("POST", "/auth/register", "", map[string]string{"email": "dup@example.com", "password": "another-pass"})
	expectError(t, resp, body, http.StatusConflict, "conflict")
}

func TestPasswordIsStoredAsBcryptHash(t *testing.T) {
	e := newEnv(t, nil)
	e.register("hash@example.com", "correct-horse")

	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var stored string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE email = ?`, "hash@example.com").Scan(&stored); err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	if stored == "correct-horse" {
		t.Fatal("password is stored in plain text")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte("correct-horse")); err != nil {
		t.Errorf("stored value is not a bcrypt hash of the password: %v", err)
	}
}

func TestLoginReturnsHS256TokenValidForOneHour(t *testing.T) {
	e := newEnv(t, nil)
	id := e.register("tok@example.com", "correct-horse")
	token := e.login("tok@example.com", "correct-horse")

	var claims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		t.Fatalf("token does not verify as HS256 with the configured secret: %v", err)
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		t.Fatalf("token needs iat and exp, got %+v", claims)
	}
	if ttl := claims.ExpiresAt.Sub(claims.IssuedAt.Time); ttl != time.Hour {
		t.Errorf("exp - iat = %v, want 1h", ttl)
	}
	if claims.Subject != strconv.Itoa(int(id)) {
		t.Errorf("sub = %q, want the user id %d", claims.Subject, int(id))
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	e := newEnv(t, nil)
	e.register("real@example.com", "correct-horse")

	r1, b1 := e.do("POST", "/auth/login", "", map[string]string{"email": "real@example.com", "password": "wrong-password"})
	expectError(t, r1, b1, http.StatusUnauthorized, "unauthorized")
	r2, b2 := e.do("POST", "/auth/login", "", map[string]string{"email": "ghost@example.com", "password": "wrong-password"})
	expectError(t, r2, b2, http.StatusUnauthorized, "unauthorized")

	if string(b1) != string(b2) {
		t.Errorf("unknown user and wrong password differ:\n%s\n%s", b1, b2)
	}
}

// --- token checks -----------------------------------------------------------------

func TestInvalidTokensAreRejected(t *testing.T) {
	e := newEnv(t, nil)
	e.signup("auth@example.com")

	sign := func(method jwt.SigningMethod, key any, exp time.Time) string {
		claims := jwt.RegisteredClaims{Subject: "1", IssuedAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))}
		if !exp.IsZero() {
			claims.ExpiresAt = jwt.NewNumericDate(exp)
		}
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	future := time.Now().Add(time.Hour)
	tests := []struct{ name, header string }{
		{"no header", ""},
		{"not a bearer scheme", "Basic dXNlcjpwYXNz"},
		{"garbage token", "Bearer not.a.jwt"},
		{"signed with another secret", "Bearer " + sign(jwt.SigningMethodHS256, []byte("another-secret-another-secret!!"), future)},
		{"expired", "Bearer " + sign(jwt.SigningMethodHS256, secret, time.Now().Add(-time.Hour))},
		{"alg none", "Bearer " + sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, future)},
		{"HS384 instead of HS256", "Bearer " + sign(jwt.SigningMethodHS384, secret, future)},
		{"no exp claim", "Bearer " + sign(jwt.SigningMethodHS256, secret, time.Time{})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var headers []string
			if tc.header != "" {
				headers = []string{"Authorization", tc.header}
			}
			resp, body := e.do("POST", "/links", "", map[string]string{"url": "https://example.com"}, headers...)
			expectError(t, resp, body, http.StatusUnauthorized, "unauthorized")
		})
	}
}

// --- creating links ------------------------------------------------------------------

func TestCreateLinkGeneratesSevenCharacterSlug(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("gen@example.com")

	slugs := map[string]bool{}
	for range 3 {
		resp, body := e.createLink(token, map[string]string{"url": "https://example.com/page"})
		expect(t, resp, body, http.StatusCreated)
		got := obj(t, body)
		slug, _ := got["slug"].(string)
		if !regexp.MustCompile(`^[A-Za-z0-9]{7}$`).MatchString(slug) {
			t.Fatalf("slug = %q, want 7 characters from [A-Za-z0-9]", slug)
		}
		if got["url"] != "https://example.com/page" {
			t.Errorf("url = %v", got["url"])
		}
		slugs[slug] = true
	}
	if len(slugs) != 3 {
		t.Errorf("generated slugs are not distinct: %v", slugs)
	}
}

func TestCreateLinkAcceptsCustomSlug(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("custom@example.com")

	for _, slug := range []string{"abc", "My_Link-2", strings.Repeat("a", 32)} {
		resp, body := e.createLink(token, map[string]string{"url": "http://example.com", "slug": slug})
		expect(t, resp, body, http.StatusCreated)
		if got := obj(t, body)["slug"]; got != slug {
			t.Errorf("slug = %v, want %q", got, slug)
		}
	}
}

func TestCreateLinkValidation(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("val@example.com")

	tests := []struct {
		name    string
		payload map[string]string
		field   string
	}{
		{"missing url", map[string]string{}, "url"},
		{"relative url", map[string]string{"url": "/just/a/path"}, "url"},
		{"url without scheme", map[string]string{"url": "example.com"}, "url"},
		{"ftp scheme", map[string]string{"url": "ftp://example.com/file"}, "url"},
		{"javascript scheme", map[string]string{"url": "javascript:alert(1)"}, "url"},
		{"url without host", map[string]string{"url": "https://"}, "url"},
		{"slug too short", map[string]string{"url": "https://example.com", "slug": "ab"}, "slug"},
		{"slug too long", map[string]string{"url": "https://example.com", "slug": strings.Repeat("a", 33)}, "slug"},
		{"slug with a space", map[string]string{"url": "https://example.com", "slug": "has space"}, "slug"},
		{"slug with a slash", map[string]string{"url": "https://example.com", "slug": "a/b/c"}, "slug"},
		{"empty slug", map[string]string{"url": "https://example.com", "slug": ""}, "slug"},
		{"reserved slug", map[string]string{"url": "https://example.com", "slug": "links"}, "slug"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := e.createLink(token, tc.payload)
			fields := expectError(t, resp, body, http.StatusUnprocessableEntity, "validation_failed")
			if msg, _ := fields[tc.field].(string); msg == "" {
				t.Errorf("fields = %v, want a message for %q", fields, tc.field)
			}
		})
	}
}

func TestDuplicateSlugIs409AndKeepsTheOriginal(t *testing.T) {
	e := newEnv(t, nil)
	alice := e.signup("alice@example.com")
	bob := e.signup("bob@example.com")
	e.mustCreate(alice, "taken", "https://example.com/alice")

	resp, body := e.createLink(bob, map[string]string{"url": "https://example.com/bob", "slug": "taken"})
	expectError(t, resp, body, http.StatusConflict, "conflict")

	resp, _ = e.do("GET", "/taken", "", nil)
	if got := resp.Header.Get("Location"); got != "https://example.com/alice" {
		t.Errorf("Location = %q, the original link was overwritten", got)
	}
}

func TestMalformedJSONIs400(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("json@example.com")
	for _, path := range []string{"/auth/register", "/auth/login", "/links"} {
		t.Run(path, func(t *testing.T) {
			resp, body := e.do("POST", path, token, "{not json")
			expectError(t, resp, body, http.StatusBadRequest, "bad_request")
		})
	}
}

// --- redirecting ------------------------------------------------------------------------

func TestRedirectIs302WithLocation(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("redir@example.com")
	target := "https://example.com/a?b=1&c=two#frag"
	e.mustCreate(token, "go-here", target)

	resp, body := e.do("GET", "/go-here", "", nil) // public: no token
	expect(t, resp, body, http.StatusFound)
	if got := resp.Header.Get("Location"); got != target {
		t.Errorf("Location = %q, want %q", got, target)
	}
}

func TestUnknownSlugIs404(t *testing.T) {
	e := newEnv(t, nil)
	resp, body := e.do("GET", "/nothing-here", "", nil)
	expectError(t, resp, body, http.StatusNotFound, "not_found")
}

func TestConcurrentRedirectsCountEveryHit(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("hits@example.com")
	e.mustCreate(token, "popular", "https://example.com/popular")

	const visits = 20
	var wg sync.WaitGroup
	for range visits {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := e.client.Get(e.url + "/popular")
			if err != nil {
				t.Errorf("GET /popular: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusFound {
				t.Errorf("status = %d, want 302", resp.StatusCode)
			}
		}()
	}
	wg.Wait()

	resp, body := e.do("GET", "/links", token, nil)
	expect(t, resp, body, http.StatusOK)
	items, _ := obj(t, body)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["hits"] != float64(visits) {
		t.Errorf("items = %v, want one link with hits = %d", items, visits)
	}
}

// --- listing and deleting -----------------------------------------------------------------

func TestListLinksReturnsOnlyOwnLinksWithHits(t *testing.T) {
	e := newEnv(t, nil)
	alice := e.signup("alice@example.com")
	bob := e.signup("bob@example.com")
	carol := e.signup("carol@example.com")
	e.mustCreate(alice, "alice-1", "https://example.com/1")
	e.mustCreate(bob, "bob-1", "https://example.com/2")
	e.do("GET", "/alice-1", "", nil)
	e.do("GET", "/alice-1", "", nil)

	resp, body := e.do("GET", "/links", alice, nil)
	expect(t, resp, body, http.StatusOK)
	got := obj(t, body)
	items, _ := got["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want only alice's link", items)
	}
	item := items[0].(map[string]any)
	if item["slug"] != "alice-1" || item["url"] != "https://example.com/1" || item["hits"] != float64(2) {
		t.Errorf("item = %v, want slug alice-1, its url and 2 hits", item)
	}
	if got["total"] != float64(1) || got["limit"] != float64(20) || got["offset"] != float64(0) {
		t.Errorf("envelope = %v, want total 1, limit 20, offset 0", got)
	}

	_, body = e.do("GET", "/links", carol, nil)
	if !strings.Contains(string(body), `"items":[]`) {
		t.Errorf("body = %s, want an empty array, not null", body)
	}
	resp, body = e.do("GET", "/links", "", nil)
	expectError(t, resp, body, http.StatusUnauthorized, "unauthorized")
}

func TestListLinksPaginates(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("page@example.com")
	for _, slug := range []string{"one", "two", "three"} {
		e.mustCreate(token, slug, "https://example.com/"+slug)
	}
	slugs := func(raw []byte) []string {
		var out []string
		items, _ := obj(t, raw)["items"].([]any)
		for _, it := range items {
			out = append(out, it.(map[string]any)["slug"].(string))
		}
		return out
	}

	_, body := e.do("GET", "/links?limit=2", token, nil)
	if got := strings.Join(slugs(body), ","); got != "three,two" {
		t.Errorf("first page = %s, want newest first: three,two", got)
	}
	if obj(t, body)["total"] != float64(3) {
		t.Errorf("total = %v, want 3", obj(t, body)["total"])
	}
	_, body = e.do("GET", "/links?limit=2&offset=2", token, nil)
	if got := strings.Join(slugs(body), ","); got != "one" {
		t.Errorf("second page = %s, want one", got)
	}
	_, body = e.do("GET", "/links?limit=1000", token, nil)
	if got := obj(t, body)["limit"]; got != float64(100) {
		t.Errorf("limit = %v, want it clamped to 100", got)
	}
	for _, q := range []string{"limit=abc", "limit=-1", "offset=-5"} {
		resp, body := e.do("GET", "/links?"+q, token, nil)
		expectError(t, resp, body, http.StatusBadRequest, "bad_request")
	}
}

func TestOwnerCanDeleteLink(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("del@example.com")
	e.mustCreate(token, "bye", "https://example.com/bye")

	resp, body := e.do("DELETE", "/links/bye", token, nil)
	expect(t, resp, body, http.StatusNoContent)
	resp, body = e.do("GET", "/bye", "", nil)
	expectError(t, resp, body, http.StatusNotFound, "not_found")
}

func TestOtherUsersCannotDeleteLink(t *testing.T) {
	e := newEnv(t, nil)
	alice := e.signup("alice@example.com")
	bob := e.signup("bob@example.com")
	e.mustCreate(alice, "mine", "https://example.com/mine")

	resp, body := e.do("DELETE", "/links/mine", bob, nil)
	expectError(t, resp, body, http.StatusForbidden, "forbidden")
	resp, body = e.do("DELETE", "/links/mine", "", nil)
	expectError(t, resp, body, http.StatusUnauthorized, "unauthorized")

	resp, body = e.do("GET", "/mine", "", nil)
	expect(t, resp, body, http.StatusFound)
}

func TestDeleteUnknownLinkIs404(t *testing.T) {
	e := newEnv(t, nil)
	token := e.signup("none@example.com")
	resp, body := e.do("DELETE", "/links/ghost", token, nil)
	expectError(t, resp, body, http.StatusNotFound, "not_found")
}

// --- rate limiting --------------------------------------------------------------------------

func slowRefill(c *shortener.Config) {
	c.RateBurst = 2
	c.RateRefillPerSecond = 0.001 // effectively no refill during a test
}

func TestCreateLinkRateLimitReturns429WithRetryAfter(t *testing.T) {
	e := newEnv(t, slowRefill)
	token := e.signup("rate@example.com")

	for i := range 2 {
		resp, body := e.createLink(token, map[string]string{"url": "https://example.com/" + strconv.Itoa(i)})
		expect(t, resp, body, http.StatusCreated)
	}
	resp, body := e.createLink(token, map[string]string{"url": "https://example.com/third"})
	expectError(t, resp, body, http.StatusTooManyRequests, "rate_limited")
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || secs < 1 {
		t.Errorf("Retry-After = %q, want a whole number of seconds >= 1", resp.Header.Get("Retry-After"))
	}

	// Only POST /links is limited.
	resp, body = e.do("GET", "/links", token, nil)
	expect(t, resp, body, http.StatusOK)
}

func TestRateLimitIsPerUser(t *testing.T) {
	e := newEnv(t, slowRefill)
	alice := e.signup("alice@example.com")
	bob := e.signup("bob@example.com")
	for i := range 3 {
		e.createLink(alice, map[string]string{"url": "https://example.com/" + strconv.Itoa(i)})
	}

	resp, body := e.createLink(bob, map[string]string{"url": "https://example.com/bob"})
	expect(t, resp, body, http.StatusCreated)
}

// --- structured logging -----------------------------------------------------------------------

func TestOneJSONLogLinePerRequest(t *testing.T) {
	e := newEnv(t, nil)
	id := e.register("log@example.com", "correct-horse")
	token := e.login("log@example.com", "correct-horse")
	e.mustCreate(token, "logged", "https://example.com")
	e.do("GET", "/missing-slug", "", nil)

	lines := e.logs.lines(t, 4)
	want := []struct {
		method, path string
		status       float64
		user         bool
	}{
		{"POST", "/auth/register", 201, false},
		{"POST", "/auth/login", 200, false},
		{"POST", "/links", 201, true},
		{"GET", "/missing-slug", 404, false},
	}
	for i, w := range want {
		l := lines[i]
		if l["method"] != w.method || l["path"] != w.path || l["status"] != w.status {
			t.Errorf("line %d = %v, want %s %s -> %v", i, l, w.method, w.path, w.status)
		}
		if rid, _ := l["request_id"].(string); rid == "" {
			t.Errorf("line %d has no request_id: %v", i, l)
		}
		if d, ok := l["duration_ms"].(float64); !ok || d < 0 {
			t.Errorf("line %d has no numeric duration_ms: %v", i, l)
		}
		gotUser, hasUser := l["user_id"]
		switch {
		case w.user && gotUser != id:
			t.Errorf("line %d user_id = %v, want %v", i, gotUser, id)
		case !w.user && hasUser:
			t.Errorf("line %d has user_id %v for an anonymous request", i, gotUser)
		}
	}
}

func TestLogsNeverContainSecrets(t *testing.T) {
	e := newEnv(t, nil)
	const password = "hunter2-hunter2"
	e.register("sec@example.com", password)
	token := e.login("sec@example.com", password)
	e.mustCreate(token, "secret-free", "https://example.com/?token=abc123")
	e.do("POST", "/auth/login", "", map[string]string{"email": "sec@example.com", "password": "wrong-" + password})
	e.do("GET", "/links", "garbage.token.value", nil)
	e.logs.lines(t, 5)

	out := e.logs.String()
	for _, secret := range []string{password, token, "Bearer", "garbage.token.value", "abc123"} {
		if strings.Contains(out, secret) {
			t.Errorf("log output contains %q:\n%s", secret, out)
		}
	}
}

func TestRequestIDIsEchoedOrGenerated(t *testing.T) {
	e := newEnv(t, nil)

	resp, _ := e.do("GET", "/nope", "", nil, "X-Request-ID", "client-id-42")
	if got := resp.Header.Get("X-Request-ID"); got != "client-id-42" {
		t.Errorf("X-Request-ID = %q, want the client's id echoed", got)
	}
	r1, _ := e.do("GET", "/nope", "", nil)
	r2, _ := e.do("GET", "/nope", "", nil)
	id1, id2 := r1.Header.Get("X-Request-ID"), r2.Header.Get("X-Request-ID")
	if id1 == "" || id1 == id2 {
		t.Errorf("generated ids = %q and %q, want two different non-empty values", id1, id2)
	}

	lines := e.logs.lines(t, 3)
	if lines[0]["request_id"] != "client-id-42" || lines[1]["request_id"] != id1 {
		t.Errorf("logged request ids %v / %v do not match the response headers", lines[0]["request_id"], lines[1]["request_id"])
	}
}
