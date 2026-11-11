package starter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SanitizeFilename turns a client-supplied filename into a safe display name.
// It is only ever used for metadata and the Content-Disposition header, never
// as a path, but it must still be harmless:
//
//   - keep only what follows the last "/" or "\" (so "../../etc/passwd" -> "passwd")
//   - drop control characters and double quotes
//   - trim surrounding spaces
//   - a result that is empty or only dots becomes "file"
//   - at most 100 characters (runes)
func SanitizeFilename(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if strings.Trim(name, ".") == "" {
		return "file"
	}
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}
	return name
}
