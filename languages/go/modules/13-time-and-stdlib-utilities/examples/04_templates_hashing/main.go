// text/template for notifications and the crypto primitives you reach for daily: SHA-256, HMAC, constant-time compare.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"
)

type Task struct {
	Title string
	Due   time.Time
	Tags  []string
}

const digestText = `Hi {{.Name}},
{{- with .Tasks}}
you have {{len .}} task{{if ne (len .) 1}}s{{end}}:
{{- range $i, $t := .}}
{{add $i 1}}. {{$t.Title | printf "%-12s"}} due {{$t.Due.Format "2006-01-02"}}{{if $t.Tags}} [{{join $t.Tags ", "}}]{{end}}
{{- end}}
{{- else}}
nothing is due.
{{- end}}
`

func main() {
	funcs := template.FuncMap{
		"add":  func(a, b int) int { return a + b },
		"join": strings.Join,
	}
	// Option missingkey=error turns a typo such as {{.Nmae}} into an error instead of "<no value>".
	tmpl := template.Must(template.New("digest").Funcs(funcs).Option("missingkey=error").Parse(digestText))

	data := map[string]any{
		"Name": "Ada",
		"Tasks": []Task{
			{"Write docs", time.Date(2030, 1, 31, 0, 0, 0, 0, time.UTC), []string{"docs", "p1"}},
			{"Ship it", time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC), nil},
		},
	}
	if err := tmpl.Execute(os.Stdout, data); err != nil {
		panic(err)
	}
	if err := tmpl.Execute(os.Stdout, map[string]any{"Name": "Bob", "Tasks": []Task{}}); err != nil {
		panic(err)
	}
	// Execute streams output, so render into a buffer when a failure must not leak half a page.
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, map[string]any{"Nmae": "typo"})
	fmt.Printf("missing key: %v (partial output %q)\n", err, buf.String())

	// text/template does NOT escape. Use html/template (module 11) for HTML.
	_, err = template.New("bad").Parse("{{.Title")
	fmt.Println("parse error:", err)

	// SHA-256: integrity and content addressing. Not for passwords (use bcrypt).
	sum := sha256.Sum256([]byte("hello"))
	fmt.Println("sha256:", hex.EncodeToString(sum[:]))
	h := sha256.New() // streaming form, for readers and large inputs
	h.Write([]byte("hel"))
	h.Write([]byte("lo"))
	fmt.Println("streamed equal:", hex.EncodeToString(h.Sum(nil)) == hex.EncodeToString(sum[:]))

	// HMAC: a keyed hash proves the message came from someone who knows the key.
	key := []byte("server-secret")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("task:42"))
	tag := mac.Sum(nil)
	fmt.Println("hmac:", base64.RawURLEncoding.EncodeToString(tag))

	verify := hmac.New(sha256.New, key)
	verify.Write([]byte("task:42"))
	fmt.Println("hmac.Equal:", hmac.Equal(tag, verify.Sum(nil)))
	fmt.Println("same tag:", subtle.ConstantTimeCompare(tag, tag) == 1, "| truncated tag:", subtle.ConstantTimeCompare(tag, tag[:31]) == 1)
	forged := append([]byte(nil), tag...)
	forged[0] ^= 1
	fmt.Println("forged accepted:", hmac.Equal(tag, forged))
}
