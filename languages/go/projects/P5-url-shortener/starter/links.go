package shortener

import "net/http"

// reservedSlugs would be shadowed by real routes (GET /links, /auth/...).
var reservedSlugs = map[string]bool{"auth": true, "links": true}

// validateURL returns "" when raw is an absolute http/https URL (with a host,
// at most 2048 bytes), otherwise a message for the "url" field error.
func validateURL(raw string) string {
	// TODO
	return ""
}

// validateSlug returns "" for 3-32 characters from [A-Za-z0-9_-] that are not
// reserved, otherwise a message for the "slug" field error.
func validateSlug(slug string) string {
	// TODO
	return ""
}

// newSlug returns 7 random characters from [A-Za-z0-9] using crypto/rand.
func newSlug() (string, error) {
	// TODO
	return "", errTODO
}

// handleCreateLink: POST /links {"url","slug"?} (auth + rate limited)
// TODO: 400 malformed JSON; 422 with fields url/slug; generated slug when
// absent (retry a few times on a collision); 409 "conflict" when a custom slug
// is taken; 201 {"slug","url"}; the owner is infoFrom(r.Context()).UserID.
func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// handleRedirect: GET /{slug} (public) -> 302 Location, counts a hit, 404 if unknown.
func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// handleListLinks: GET /links?limit=&offset= (auth) -> the caller's links.
// TODO: {"items":[{"slug","url","hits"}],"total":N,"limit":L,"offset":O};
// limit default 20, max 100 (clamp), offset default 0; non-numeric or
// negative values -> 400 "bad_request"; items must be [] (not null) when empty.
func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// handleDeleteLink: DELETE /links/{slug} (auth)
// TODO: 404 unknown, 403 "forbidden" when the link belongs to someone else, 204 on success.
func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}
