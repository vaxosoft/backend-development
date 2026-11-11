package shortener

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.BcryptCost)
	return string(hash), err
}

func (s *Server) checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *Server) issueToken(userID int64) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.TokenTTL)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.cfg.JWTSecret)
}

func (s *Server) parseToken(token string) (int64, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return s.cfg.JWTSecret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // never trust the alg header
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(claims.Subject, 10, 64)
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decodeJSON(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))

	fields := map[string]string{}
	if !strings.Contains(email, "@") {
		fields["email"] = "must be a valid e-mail address"
	}
	if n := len(in.Password); n < 8 || n > 72 {
		fields["password"] = "must be 8-72 bytes"
	}
	if len(fields) > 0 {
		writeValidation(w, "invalid registration", fields)
		return
	}

	hash, err := s.hashPassword(in.Password)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), email, hash)
	if errors.Is(err, ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", "e-mail is already registered")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": user.ID, "email": user.Email})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decodeJSON(w, r, &in) {
		return
	}
	user, err := s.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil && !errors.Is(err, ErrNotFound) {
		s.internal(w, r, err)
		return
	}

	// Always run one bcrypt comparison so an unknown e-mail takes as long as a wrong password.
	hash := s.dummyHash
	if err == nil {
		hash = user.PasswordHash
	}
	if !s.checkPassword(hash, in.Password) || err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid e-mail or password")
		return
	}

	token, err := s.issueToken(user.ID)
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
