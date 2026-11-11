// Example 01: password hashing with bcrypt, random tokens, and constant-time comparison.
//
//	go run ./modules/11-auth-and-security-basics/examples/01_passwords_and_secrets
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// newToken returns n random bytes as hex. Never use math/rand for secrets.
func newToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the OS random source failing is not recoverable
	}
	return hex.EncodeToString(b)
}

func fingerprint(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

func main() {
	password := []byte("correct horse battery staple")

	h1, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	h2, _ := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	cost, _ := bcrypt.Cost(h1)
	fmt.Println("hash prefix:", string(h1[:7]))
	fmt.Println("hash length:", len(h1))
	fmt.Println("cost:", cost)
	fmt.Println("same password, different hashes:", string(h1) != string(h2))

	fmt.Println("right password:", bcrypt.CompareHashAndPassword(h1, password))
	err = bcrypt.CompareHashAndPassword(h1, []byte("wrong"))
	fmt.Println("wrong password is mismatch:", errors.Is(err, bcrypt.ErrMismatchedHashAndPassword))

	_, err = bcrypt.GenerateFromPassword([]byte(strings.Repeat("x", 73)), bcrypt.DefaultCost)
	fmt.Println("73-byte password too long:", errors.Is(err, bcrypt.ErrPasswordTooLong))

	// API keys are random and high-entropy, so a fast hash is enough to store them.
	apiKey := newToken(32)
	stored := fingerprint(apiKey)
	fmt.Println("api key length:", len(apiKey))
	fmt.Println("two tokens differ:", newToken(16) != newToken(16))

	good := subtle.ConstantTimeCompare(fingerprint(apiKey), stored) == 1
	bad := subtle.ConstantTimeCompare(fingerprint("guess"), stored) == 1
	fmt.Println("presented key accepted:", good)
	fmt.Println("guessed key accepted:", bad)
}
