// Example 03: untrusted input: SQL injection, HTML escaping (XSS) and allow-list validation.
//
//	go run ./modules/11-auth-and-security-basics/examples/03_untrusted_input
package main

import (
	"database/sql"
	"fmt"
	htmltemplate "html/template"
	"os"
	"regexp"
	"strings"
	texttemplate "text/template"

	_ "modernc.org/sqlite"
)

// findUnsafe builds SQL by concatenation. NEVER do this; it is here to be attacked.
func findUnsafe(db *sql.DB, email string) ([]string, error) {
	return names(db, "SELECT name FROM users WHERE email = '"+email+"' ORDER BY id")
}

func findSafe(db *sql.DB, email string) ([]string, error) {
	return names(db, "SELECT name FROM users WHERE email = ? ORDER BY id", email)
}

func names(db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

var tagRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,29}$`)

func validTag(s string) bool { return tagRe.MatchString(s) }

func main() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // every in-memory SQLite connection is its own database
	for _, q := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)`,
		`INSERT INTO users (name, email) VALUES ('Ada', 'ada@example.com'), ('Linus', 'linus@example.com')`,
	} {
		if _, err := db.Exec(q); err != nil {
			panic(err)
		}
	}

	attack := "nobody' OR '1'='1"
	got, _ := findUnsafe(db, attack)
	fmt.Println("concatenated, attack:", got)
	got, _ = findSafe(db, attack)
	fmt.Println("parameterised, attack:", got)
	got, _ = findSafe(db, "ada@example.com")
	fmt.Println("parameterised, honest:", got)
	_, err = findUnsafe(db, "x'")
	fmt.Println("concatenated, stray quote breaks the query:", err != nil)

	title := `<script>alert("xss")</script>`
	const page = `<h1>{{.Title}}</h1><a href="{{.URL}}">link</a>`
	data := map[string]string{"Title": title, "URL": "javascript:alert(1)"}

	fmt.Print("text/template: ")
	if err := texttemplate.Must(texttemplate.New("p").Parse(page+"\n")).Execute(os.Stdout, data); err != nil {
		panic(err)
	}
	fmt.Print("html/template: ")
	if err := htmltemplate.Must(htmltemplate.New("p").Parse(page+"\n")).Execute(os.Stdout, data); err != nil {
		panic(err)
	}

	for _, tag := range []string{"docs", "p1", "Docs", "a b", "", strings.Repeat("x", 31)} {
		fmt.Printf("tag %q valid: %t\n", tag, validTag(tag))
	}
}
