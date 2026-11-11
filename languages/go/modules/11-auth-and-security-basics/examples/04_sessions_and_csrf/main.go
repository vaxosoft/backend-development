// Example 04: server-side sessions in cookies plus CSRF protection, exercised through a cookie-aware client.
//
//	go run ./modules/11-auth-and-security-basics/examples/04_sessions_and_csrf
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
)

type session struct{ user, csrf string }

type sessions struct {
	mu   sync.Mutex
	byID map[string]session
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *sessions) create(user string) (id string, sess session) {
	id, sess = randomHex(32), session{user: user, csrf: randomHex(16)}
	s.mu.Lock()
	s.byID[id] = sess
	s.mu.Unlock()
	return id, sess
}

func (s *sessions) lookup(r *http.Request) (session, bool) {
	c, err := r.Cookie("sid")
	if err != nil {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[c.Value]
	return sess, ok
}

func (s *sessions) destroy(id string) {
	s.mu.Lock()
	delete(s.byID, id)
	s.mu.Unlock()
}

// requireSession rejects anonymous callers and, for unsafe methods, requests without the CSRF token.
func (s *sessions) requireSession(next func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.lookup(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			sent := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(sent), []byte(sess.csrf)) != 1 {
				http.Error(w, "csrf token missing or wrong", http.StatusForbidden)
				return
			}
		}
		next(w, r, sess)
	}
}

func newHandler() http.Handler {
	s := &sessions{byID: map[string]session{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("user") != "ada" || r.FormValue("password") != "s3cret-pass" {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		id, sess := s.create("ada")
		http.SetCookie(w, &http.Cookie{
			Name: "sid", Value: id, Path: "/",
			HttpOnly: true,                 // invisible to JavaScript, limits XSS damage
			SameSite: http.SameSiteLaxMode, // not sent on cross-site POSTs
			Secure:   false,                // set true behind HTTPS; cookiejar would not send it over plain HTTP
		})
		fmt.Fprint(w, sess.csrf)
	})
	mux.HandleFunc("GET /me", s.requireSession(func(w http.ResponseWriter, r *http.Request, sess session) {
		fmt.Fprint(w, "hello "+sess.user)
	}))
	mux.HandleFunc("POST /tasks", s.requireSession(func(w http.ResponseWriter, r *http.Request, sess session) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "task created by "+sess.user)
	}))
	mux.HandleFunc("POST /logout", s.requireSession(func(w http.ResponseWriter, r *http.Request, sess session) {
		c, _ := r.Cookie("sid")
		s.destroy(c.Value)
		http.SetCookie(w, &http.Cookie{Name: "sid", Path: "/", MaxAge: -1, HttpOnly: true})
		w.WriteHeader(http.StatusNoContent)
	}))
	return mux
}

func main() {
	srv := httptest.NewServer(newHandler())
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	do := func(label, method, path, body, csrf string) string {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("%-24s %d %s\n", label, resp.StatusCode, strings.TrimSpace(string(b)))
		return string(b)
	}

	do("me before login", "GET", "/me", "", "")
	do("login wrong password", "POST", "/login", "user=ada&password=nope", "")

	req, _ := http.NewRequest("POST", srv.URL+"/login", strings.NewReader("user=ada&password=s3cret-pass"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	csrfBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	csrf := string(csrfBytes)
	c := resp.Cookies()[0]
	fmt.Printf("login cookie: name=%s httponly=%t samesite-lax=%t path=%s\n",
		c.Name, c.HttpOnly, c.SameSite == http.SameSiteLaxMode, c.Path)

	do("me after login", "GET", "/me", "", "")
	do("post without csrf", "POST", "/tasks", "", "")
	do("post with wrong csrf", "POST", "/tasks", "", "guess")
	do("post with csrf", "POST", "/tasks", "", csrf)
	do("logout", "POST", "/logout", "", csrf)
	do("me after logout", "GET", "/me", "", "")
}
