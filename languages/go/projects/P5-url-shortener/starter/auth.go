package shortener

import "net/http"

func notImplemented(w http.ResponseWriter) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "TODO")
}

func (s *Server) hashPassword(password string) (string, error) {
	// TODO: bcrypt with s.cfg.BcryptCost.
	return "", errTODO
}

func (s *Server) checkPassword(hash, password string) bool {
	// TODO: bcrypt.CompareHashAndPassword.
	return false
}

func (s *Server) issueToken(userID int64) (string, error) {
	// TODO: HS256 JWT with subject = user id, iat, and exp = now + s.cfg.TokenTTL,
	// signed with s.cfg.JWTSecret.
	return "", errTODO
}

func (s *Server) parseToken(token string) (int64, error) {
	// TODO: verify the signature with s.cfg.JWTSecret, accept ONLY HS256
	// (jwt.WithValidMethods), require exp (jwt.WithExpirationRequired), and
	// return the user id from the subject.
	return 0, errTODO
}

// handleRegister: POST /auth/register {"email","password"}
// TODO: 400 malformed JSON; 422 (fields email/password) when the e-mail has no
// "@" or the password is not 8-72 bytes (bcrypt ignores bytes past 72);
// normalise the e-mail (trim + lowercase); 409 "conflict" on a duplicate;
// store only the bcrypt hash; answer 201 {"id","email"}.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// handleLogin: POST /auth/login {"email","password"} -> 200 {"token"}
// TODO: unknown e-mail and wrong password must give the SAME 401 "unauthorized"
// answer, and take the same time (compare against s.dummyHash for unknown users).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}
