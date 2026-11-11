package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"webdev/projects/P3-file-service/starter"
)

type service struct {
	t    *testing.T
	url  string
	base string // parent of the storage dir; nothing may be written outside dir
	dir  string
}

func newService(t *testing.T, maxSize int64) *service {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "store")
	return startService(t, base, dir, maxSize)
}

func startService(t *testing.T, base, dir string, maxSize int64) *service {
	t.Helper()
	h, err := starter.New(starter.Config{Dir: dir, MaxSize: maxSize})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &service{t: t, url: srv.URL, base: base, dir: dir}
}

type metaJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
}

type upload struct {
	field       string // multipart field name, default "file"
	filename    string
	contentType string // default application/octet-stream
	content     []byte
	before      map[string]string // plain form fields sent before the file
}

func (u upload) body() (*bytes.Buffer, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range u.before {
		_ = mw.WriteField(k, v)
	}
	field := u.field
	if field == "" {
		field = "file"
	}
	ct := u.contentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, u.filename))
	h.Set("Content-Type", ct)
	part, _ := mw.CreatePart(h)
	_, _ = part.Write(u.content)
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func (s *service) post(u upload) (*http.Response, []byte) {
	s.t.Helper()
	body, ct := u.body()
	return s.postRaw(body, ct)
}

func (s *service) postRaw(body io.Reader, contentType string) (*http.Response, []byte) {
	s.t.Helper()
	resp, err := http.Post(s.url+"/files", contentType, body)
	if err != nil {
		s.t.Fatalf("POST /files: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	return resp, raw
}

func (s *service) upload(u upload) metaJSON {
	s.t.Helper()
	resp, raw := s.post(u)
	if resp.StatusCode != http.StatusCreated {
		s.t.Fatalf("upload %q: status %d, body %s", u.filename, resp.StatusCode, raw)
	}
	var m metaJSON
	if err := json.Unmarshal(raw, &m); err != nil {
		s.t.Fatalf("decode %s: %v", raw, err)
	}
	return m
}

func (s *service) do(method, path string, headers ...string) (*http.Response, []byte) {
	s.t.Helper()
	req, err := http.NewRequest(method, s.url+path, nil)
	if err != nil {
		s.t.Fatal(err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	return resp, raw
}

// files lists every regular file under root, relative to root.
func files(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// payload returns n deterministic, non-repeating-looking bytes.
func payload(n int) []byte {
	b := make([]byte, n)
	x := uint32(2463534242)
	for i := range b {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		b[i] = byte(x)
	}
	return b
}

func assertError(t *testing.T, resp *http.Response, raw []byte, status int, code string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d (body %s)", resp.StatusCode, status, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(string(raw), `"code":"`+code+`"`) {
		t.Errorf("body = %s, want error code %q", raw, code)
	}
}

// --- upload -----------------------------------------------------------------

func TestUploadReturns201WithMetadataAndLocation(t *testing.T) {
	s := newService(t, 0)
	content := []byte("hello, file service\n")
	resp, raw := s.post(upload{filename: "notes.txt", contentType: "text/plain", content: content})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	var m metaJSON
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(m.ID) {
		t.Errorf("id = %q, want 32 lowercase hex characters", m.ID)
	}
	if m.Name != "notes.txt" || m.Size != int64(len(content)) || m.SHA256 != sum(content) || m.ContentType != "text/plain" {
		t.Errorf("metadata = %+v", m)
	}
	if loc := resp.Header.Get("Location"); loc != "/files/"+m.ID {
		t.Errorf("Location = %q", loc)
	}
	for _, key := range []string{`"id"`, `"name"`, `"size"`, `"sha256"`, `"content_type"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("response lacks %s: %s", key, raw)
		}
	}
}

func TestUploadWithoutContentTypeDefaultsToOctetStream(t *testing.T) {
	s := newService(t, 0)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="blob.bin"`)
	part, _ := mw.CreatePart(h)
	_, _ = part.Write([]byte{1, 2, 3})
	_ = mw.Close()

	resp, raw := s.postRaw(&buf, mw.FormDataContentType())
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	var m metaJSON
	_ = json.Unmarshal(raw, &m)
	if m.ContentType != "application/octet-stream" {
		t.Errorf("content_type = %q", m.ContentType)
	}
}

func TestUploadSkipsOtherFieldsBeforeTheFilePart(t *testing.T) {
	s := newService(t, 0)
	m := s.upload(upload{filename: "a.txt", content: []byte("x"), before: map[string]string{"comment": "hi", "tag": "y"}})
	if m.Size != 1 {
		t.Errorf("size = %d, want 1", m.Size)
	}
}

func TestUploadAcceptsAnEmptyFile(t *testing.T) {
	s := newService(t, 0)
	m := s.upload(upload{filename: "empty.txt"})
	if m.Size != 0 || m.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("metadata = %+v", m)
	}
}

func TestUploadWithoutFilePartIs400(t *testing.T) {
	s := newService(t, 0)
	resp, raw := s.post(upload{field: "attachment", filename: "a.txt", content: []byte("x")})
	assertError(t, resp, raw, 400, "bad_request")
}

func TestUploadThatIsNotMultipartIs400(t *testing.T) {
	s := newService(t, 0)
	resp, raw := s.postRaw(strings.NewReader(`{"file":"x"}`), "application/json")
	assertError(t, resp, raw, 400, "bad_request")
}

func TestEachUploadGetsItsOwnID(t *testing.T) {
	s := newService(t, 0)
	a := s.upload(upload{filename: "same.txt", content: []byte("one")})
	b := s.upload(upload{filename: "same.txt", content: []byte("two")})
	if a.ID == b.ID {
		t.Fatal("two uploads share an id")
	}
	if _, body := s.do("GET", "/files/"+a.ID); string(body) != "one" {
		t.Errorf("first file = %q", body)
	}
	if _, body := s.do("GET", "/files/"+b.ID); string(body) != "two" {
		t.Errorf("second file = %q", body)
	}
}

// --- size limits ------------------------------------------------------------

func TestUploadOverTheConfiguredLimitIs413AndLeavesNoFiles(t *testing.T) {
	s := newService(t, 1024)
	resp, raw := s.post(upload{filename: "big.bin", content: payload(2048)})
	assertError(t, resp, raw, 413, "payload_too_large")
	if left := files(t, s.dir); len(left) != 0 {
		t.Errorf("files left behind after a rejected upload: %v", left)
	}
}

func TestUploadExactlyAtTheLimitIsAcceptedOneByteMoreIsNot(t *testing.T) {
	s := newService(t, 1024)
	if m := s.upload(upload{filename: "ok.bin", content: payload(1024)}); m.Size != 1024 {
		t.Errorf("size = %d", m.Size)
	}
	resp, raw := s.post(upload{filename: "over.bin", content: payload(1025)})
	assertError(t, resp, raw, 413, "payload_too_large")
}

func TestUploadFarOverTheLimitIs413(t *testing.T) {
	s := newService(t, 1024)
	resp, raw := s.post(upload{filename: "huge.bin", content: payload(1 << 20)})
	assertError(t, resp, raw, 413, "payload_too_large")
	if left := files(t, s.dir); len(left) != 0 {
		t.Errorf("files left behind: %v", left)
	}
}

func TestDefaultLimitIs5MiB(t *testing.T) {
	s := newService(t, 0)
	if m := s.upload(upload{filename: "max.bin", content: payload(5 << 20)}); m.Size != 5<<20 {
		t.Errorf("size = %d", m.Size)
	}
	resp, raw := s.post(upload{filename: "over.bin", content: payload(5<<20 + 1)})
	assertError(t, resp, raw, 413, "payload_too_large")
}

// --- download ---------------------------------------------------------------

func TestDownloadStreamsExactBytesWithHeaders(t *testing.T) {
	s := newService(t, 0)
	content := payload(300_000)
	m := s.upload(upload{filename: "report.pdf", contentType: "application/pdf", content: content})

	resp, body := s.do("GET", "/files/"+m.ID)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !bytes.Equal(body, content) {
		t.Error("downloaded bytes differ from the upload")
	}
	want := map[string]string{
		"Content-Type":        "application/pdf",
		"Content-Length":      fmt.Sprint(len(content)),
		"Content-Disposition": `attachment; filename="report.pdf"`,
		"ETag":                `"` + sum(content) + `"`,
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestDownloadHeaderUsesTheSanitizedName(t *testing.T) {
	s := newService(t, 0)
	m := s.upload(upload{filename: `evil"; x=y.txt`, content: []byte("x")})
	resp, _ := s.do("GET", "/files/"+m.ID)
	cd := resp.Header.Get("Content-Disposition")
	if cd != `attachment; filename="evil; x=y.txt"` {
		t.Errorf("Content-Disposition = %q (quotes must be removed)", cd)
	}
}

func TestDownloadWithMatchingIfNoneMatchIs304WithoutBody(t *testing.T) {
	s := newService(t, 0)
	content := []byte("cache me")
	m := s.upload(upload{filename: "c.txt", content: content})
	etag := `"` + sum(content) + `"`
	for _, header := range []string{etag, `"other", ` + etag, "*", "W/" + etag} {
		resp, body := s.do("GET", "/files/"+m.ID, "If-None-Match", header)
		if resp.StatusCode != http.StatusNotModified || len(body) != 0 {
			t.Errorf("If-None-Match %q: status %d, %d body bytes, want 304 and none", header, resp.StatusCode, len(body))
		}
		if got := resp.Header.Get("ETag"); got != etag {
			t.Errorf("304 must repeat the ETag, got %q", got)
		}
	}
}

func TestDownloadWithDifferentIfNoneMatchReturnsTheFile(t *testing.T) {
	s := newService(t, 0)
	m := s.upload(upload{filename: "c.txt", content: []byte("fresh")})
	resp, body := s.do("GET", "/files/"+m.ID, "If-None-Match", `"stale"`)
	if resp.StatusCode != 200 || string(body) != "fresh" {
		t.Errorf("status %d body %q", resp.StatusCode, body)
	}
}

func TestDownloadUnknownIDIs404WithEnvelope(t *testing.T) {
	s := newService(t, 0)
	resp, raw := s.do("GET", "/files/"+strings.Repeat("0", 32))
	assertError(t, resp, raw, 404, "not_found")
}

func TestMalformedAndTraversalIDsAre404(t *testing.T) {
	s := newService(t, 0)
	// A file outside the storage dir that a traversal would reach.
	if err := os.WriteFile(filepath.Join(s.base, "secret.json"), []byte(`{"name":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.base, "secret.blob"), []byte("TOP SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"..%2Fsecret", "%2e%2e%2fsecret", "not-an-id", strings.Repeat("A", 32), strings.Repeat("a", 31), strings.Repeat("a", 33)} {
		t.Run(id, func(t *testing.T) {
			for _, method := range []string{"GET", "DELETE"} {
				resp, raw := s.do(method, "/files/"+id)
				assertError(t, resp, raw, 404, "not_found")
				if strings.Contains(string(raw), "SECRET") {
					t.Fatal("leaked a file from outside the storage directory")
				}
			}
		})
	}
	if _, err := os.Stat(filepath.Join(s.base, "secret.blob")); err != nil {
		t.Errorf("a DELETE with a traversal id removed a file outside the storage dir: %v", err)
	}
}

// --- delete -----------------------------------------------------------------

func TestDeleteReturns204RemovesTheFileAndItsDiskSpace(t *testing.T) {
	s := newService(t, 0)
	m := s.upload(upload{filename: "temp.txt", content: []byte("bye")})
	if resp, body := s.do("DELETE", "/files/"+m.ID); resp.StatusCode != 204 || len(body) != 0 {
		t.Fatalf("delete: status %d, %d body bytes", resp.StatusCode, len(body))
	}
	resp, raw := s.do("GET", "/files/"+m.ID)
	assertError(t, resp, raw, 404, "not_found")
	if left := files(t, s.dir); len(left) != 0 {
		t.Errorf("files left on disk after delete: %v", left)
	}
	resp, raw = s.do("DELETE", "/files/"+m.ID)
	assertError(t, resp, raw, 404, "not_found")
}

func TestDeleteUnknownIDIs404(t *testing.T) {
	s := newService(t, 0)
	resp, raw := s.do("DELETE", "/files/"+strings.Repeat("1", 32))
	assertError(t, resp, raw, 404, "not_found")
}

// --- safety and persistence --------------------------------------------------

func TestHostileFilenameNeverEscapesTheStorageDirectory(t *testing.T) {
	s := newService(t, 0)
	for _, name := range []string{"../../etc/passwd", `..\..\windows\system.ini`, "/etc/shadow", ".."} {
		m := s.upload(upload{filename: name, content: []byte("payload")})
		if strings.ContainsAny(m.Name, `/\`) || m.Name == ".." || m.Name == "" {
			t.Errorf("name %q was not sanitized: %q", name, m.Name)
		}
	}
	for _, f := range files(t, s.base) {
		if !strings.HasPrefix(f, "store"+string(filepath.Separator)) {
			t.Errorf("file written outside the storage dir: %s", f)
		}
		if strings.Contains(strings.ToLower(f), "passwd") || strings.Contains(f, "shadow") || strings.Contains(f, "system.ini") {
			t.Errorf("client-supplied filename leaked into an on-disk name: %s", f)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"my file (1).txt", "my file (1).txt"},
		{"../../etc/passwd", "passwd"},
		{`..\..\windows\system.ini`, "system.ini"},
		{"a/b/c.txt", "c.txt"},
		{"/abs/path.txt", "path.txt"},
		{"bad\x00na\nme\t.txt", "badname.txt"},
		{`we"ird.txt`, "weird.txt"},
		{"  padded.txt  ", "padded.txt"},
		{"..", "file"},
		{".", "file"},
		{"", "file"},
		{"dir/", "file"},
		{"résumé.pdf", "résumé.pdf"},
		{".hidden", ".hidden"},
		{strings.Repeat("a", 300), strings.Repeat("a", 100)},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			if got := starter.SanitizeFilename(tc.in); got != tc.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFilesSurviveARestartOnTheSameDirectory(t *testing.T) {
	first := newService(t, 0)
	m := first.upload(upload{filename: "keep.txt", contentType: "text/plain", content: []byte("persistent")})

	second := startService(t, first.base, first.dir, 0)
	resp, body := second.do("GET", "/files/"+m.ID)
	if resp.StatusCode != 200 || string(body) != "persistent" || resp.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("after restart: status %d body %q type %q", resp.StatusCode, body, resp.Header.Get("Content-Type"))
	}
}
