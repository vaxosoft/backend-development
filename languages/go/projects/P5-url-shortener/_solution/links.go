package shortener

import (
	"crypto/rand"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// reservedSlugs would be shadowed by real routes (GET /links, /auth/...).
var reservedSlugs = map[string]bool{"auth": true, "links": true}

const slugAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// validateURL returns "" when raw is an absolute http/https URL (with a host,
// at most 2048 bytes), otherwise a message for the "url" field error.
func validateURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "must be an absolute http or https URL"
	}
	return ""
}

// validateSlug returns "" for 3-32 characters from [A-Za-z0-9_-] that are not
// reserved, otherwise a message for the "slug" field error.
func validateSlug(slug string) string {
	if n := len(slug); n < 3 || n > 32 {
		return "must be 3-32 characters"
	}
	for _, c := range slug {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
		if !ok {
			return "may only contain letters, digits, '_' and '-'"
		}
	}
	if reservedSlugs[slug] {
		return "is reserved"
	}
	return ""
}

// newSlug returns 7 random characters from [A-Za-z0-9] using crypto/rand.
func newSlug() (string, error) {
	const length = 7
	// 248 = 62*4 is the largest multiple of 62 below 256; bytes above it are
	// skipped so every character is equally likely (no modulo bias).
	const limit = 248
	slug := make([]byte, 0, length)
	buf := make([]byte, 16)
	for len(slug) < length {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b < limit && len(slug) < length {
				slug = append(slug, slugAlphabet[int(b)%len(slugAlphabet)])
			}
		}
	}
	return string(slug), nil
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL  string  `json:"url"`
		Slug *string `json:"slug"` // pointer: absent and "" are different
	}
	if !decodeJSON(w, r, &in) {
		return
	}

	fields := map[string]string{}
	if msg := validateURL(in.URL); msg != "" {
		fields["url"] = msg
	}
	if in.Slug != nil {
		if msg := validateSlug(*in.Slug); msg != "" {
			fields["slug"] = msg
		}
	}
	if len(fields) > 0 {
		writeValidation(w, "invalid link", fields)
		return
	}

	ownerID := infoFrom(r.Context()).UserID
	slug, err := s.saveLink(r, ownerID, in.URL, in.Slug)
	if errors.Is(err, ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", "slug is already taken")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"slug": slug, "url": in.URL})
}

// saveLink stores the link under the custom slug, or under a generated one,
// trying again in the unlikely case of a collision.
func (s *Server) saveLink(r *http.Request, ownerID int64, target string, custom *string) (string, error) {
	if custom != nil {
		return *custom, s.store.CreateLink(r.Context(), ownerID, *custom, target)
	}
	for range 5 {
		slug, err := newSlug()
		if err != nil {
			return "", err
		}
		err = s.store.CreateLink(r.Context(), ownerID, slug, target)
		if !errors.Is(err, ErrConflict) {
			return slug, err
		}
	}
	return "", errors.New("could not generate a free slug")
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	target, err := s.store.Hit(r.Context(), r.PathValue("slug"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "unknown link")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	limit, ok := intParam(w, r, "limit", 20)
	if !ok {
		return
	}
	offset, ok := intParam(w, r, "offset", 0)
	if !ok {
		return
	}
	limit = min(limit, 100)

	links, total, err := s.store.LinksByOwner(r.Context(), infoFrom(r.Context()).UserID, limit, offset)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": links, "total": total, "limit": limit, "offset": offset})
}

// intParam reads a non-negative integer query parameter, answering 400 itself on bad input.
func intParam(w http.ResponseWriter, r *http.Request, name string, def int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		writeError(w, http.StatusBadRequest, "bad_request", name+" must be a non-negative integer")
		return 0, false
	}
	return n, true
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	link, err := s.store.LinkBySlug(r.Context(), slug)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "unknown link")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	if link.OwnerID != infoFrom(r.Context()).UserID {
		writeError(w, http.StatusForbidden, "forbidden", "you do not own this link")
		return
	}
	if err := s.store.DeleteLink(r.Context(), slug); err != nil && !errors.Is(err, ErrNotFound) {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
