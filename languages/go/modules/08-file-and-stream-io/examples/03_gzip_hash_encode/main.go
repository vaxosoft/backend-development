// Example 03: gzip, base64, hex and SHA-256 as stream wrappers.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strings"
)

func main() {
	original := strings.Repeat("task: write docs\n", 200)

	// Compress: the gzip.Writer must be Closed to write the trailer; Flush is not enough.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := io.WriteString(zw, original); err != nil {
		log.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("original bytes:", len(original), "| smaller after gzip:", buf.Len() < len(original)/10)

	// A copy of the compressed bytes with the last 10 removed simulates a cut-off download.
	truncated := bytes.Clone(buf.Bytes()[:buf.Len()-10])

	// Decompress by wrapping the compressed reader.
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		log.Fatal(err)
	}
	back, err := io.ReadAll(zr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("round trip equal:", string(back) == original)

	// Truncated input is reported, not silently accepted.
	zr, err = gzip.NewReader(bytes.NewReader(truncated))
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.ReadAll(zr)
	fmt.Println("truncated stream:", err)

	// Encodings convert bytes to text. Neither is encryption.
	secret := []byte("user:pa55word")
	fmt.Println("base64 std:", base64.StdEncoding.EncodeToString(secret))
	fmt.Println("base64 url:", base64.RawURLEncoding.EncodeToString([]byte{0xfb, 0xff, 0xfe}))
	fmt.Println("hex:", hex.EncodeToString(secret[:4]))

	// Hashing a stream never needs the whole content in memory.
	h := sha256.New()
	if _, err := io.Copy(h, strings.NewReader("hello")); err != nil {
		log.Fatal(err)
	}
	fmt.Println("sha256(hello):", hex.EncodeToString(h.Sum(nil)))

	// Streaming base64 encoder wrapped around any writer.
	var out strings.Builder
	enc := base64.NewEncoder(base64.StdEncoding, &out)
	io.WriteString(enc, "stream me")
	enc.Close() // flushes the final partial block
	fmt.Println("encoder:", out.String())
}
