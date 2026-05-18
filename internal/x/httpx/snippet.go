package httpx

// Snippet returns a printable prefix of an HTTP response body, suitable
// for embedding in error messages. The cap is 512 bytes; oversize bodies
// are truncated with an ellipsis.
func Snippet(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
