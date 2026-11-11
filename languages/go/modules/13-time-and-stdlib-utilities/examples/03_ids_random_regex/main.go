// UUIDs, random numbers (seeded and secure) and regular expressions.
package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand/v2"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Compile once at package level; MustCompile panics on a bad pattern (a programmer error).
var (
	tagRe   = regexp.MustCompile(`#([a-z][a-z0-9-]{0,29})`)
	dueRe   = regexp.MustCompile(`(?i)\bdue (?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})\b`)
	spaceRe = regexp.MustCompile(`\s+`)
)

func main() {
	// UUIDs: v4 is random, v5 is a deterministic hash of a namespace and a name.
	id := uuid.New()
	fmt.Println("v4 version:", int(id.Version()), "len:", len(id.String()))
	parsed, err := uuid.Parse(id.String())
	fmt.Println("round trip:", parsed == id, err)
	_, err = uuid.Parse("not-a-uuid")
	fmt.Println("invalid:", err)
	ns := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("tasks.example.com"))
	fmt.Println("v5:", ns, ns == uuid.NewSHA1(uuid.NameSpaceDNS, []byte("tasks.example.com")))
	v7, err := uuid.NewV7()
	fmt.Println("v7 version:", int(v7.Version()), err)

	// math/rand/v2: fast, NOT secure. A seeded generator is reproducible (good for tests).
	r := mrand.New(mrand.NewPCG(1, 2))
	fmt.Println("seeded:", r.IntN(100), r.IntN(100), r.IntN(100))
	words := []string{"todo", "in_progress", "done"}
	r.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	fmt.Println("shuffled:", words)
	n := mrand.IntN(10) // the package-level functions are randomly seeded
	fmt.Println("global in range:", n >= 0 && n < 10)

	// crypto/rand for anything secret: tokens, session ids, password salts.
	token := rand.Text() // 26 chars of base32, 128 bits
	fmt.Println("token length:", len(token))
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		panic(err)
	}
	k, err := rand.Int(rand.Reader, big.NewInt(1000))
	fmt.Println("unbiased int in range:", k.Int64() < 1000, err)

	// Regular expressions (RE2: linear time, no backreferences or lookaround).
	text := "Write docs #docs #p1 due 2030-01-31 and   ship #Release-1"
	fmt.Println(tagRe.FindAllString(text, -1))
	fmt.Println(tagRe.FindAllStringSubmatch(text, -1)[1][1])

	m := dueRe.FindStringSubmatch(text)
	for i, name := range dueRe.SubexpNames() {
		if name != "" {
			fmt.Printf("%s=%s ", name, m[i])
		}
	}
	fmt.Println()

	fmt.Println(spaceRe.ReplaceAllString(text, " "))
	fmt.Println(tagRe.ReplaceAllStringFunc(text, strings.ToUpper))
	fmt.Println(dueRe.ReplaceAllString(text, "due $day/$month/$year"))

	_, err = regexp.Compile(`(unclosed`)
	fmt.Println("compile error:", err)
	fmt.Println("anchored:", regexp.MustCompile(`^\d+$`).MatchString("12a"), regexp.MustCompile(`\d+`).MatchString("12a"))
}
