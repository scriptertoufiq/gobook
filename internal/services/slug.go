package services

import (
	"regexp"
	"strings"
)

var (
	nonAlphaNum   = regexp.MustCompile(`[^a-z0-9]+`)
	trimSeparator = regexp.MustCompile(`^-+|-+$`)
)

// Slugify turns "Hello, World!" into "hello-world". Shared by every service
// that derives a URL key from a title or name — including the ones
// `go run ./cmd/make` generates, so this must stay in package services.
func Slugify(s string) string {
	slug := nonAlphaNum.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	return trimSeparator.ReplaceAllString(slug, "")
}
