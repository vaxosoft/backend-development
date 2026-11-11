package starter

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
	// TODO
	return name
}
