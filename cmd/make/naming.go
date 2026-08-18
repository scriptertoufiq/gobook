package main

import (
	"strings"
	"unicode"
)

// Names holds every spelling of a resource name the templates need.
// For input "blogPost" (or "blog_post", or "BlogPost") you get:
//
//	Pascal=BlogPost  Camel=blogPost  Snake=blog_post
//	Plural=BlogPosts PluralCamel=blogPosts PluralSnake=blog_posts
//	PluralKebab=blog-posts
//
// Snake spellings are for SQL (table names); kebab is for URLs.
type Names struct {
	Pascal      string
	Camel       string
	Snake       string
	Plural      string
	PluralCamel string
	PluralSnake string
	PluralKebab string
	Module      string

	// MigrationID is the timestamped identity of a migration file — both its
	// name on disk and the key recorded in the ledger, so the two can never
	// drift apart. Set by main once the sub-command is known, since the same
	// resource name yields `create_posts_table` from `scaffold` and a
	// free-form `add_status_to_posts` from `migration`.
	MigrationID string
}

func newNames(raw, module string) Names {
	words := splitWords(raw)
	if len(words) == 0 {
		return Names{}
	}

	plural := make([]string, len(words))
	copy(plural, words)
	plural[len(plural)-1] = pluralize(plural[len(plural)-1])

	return Names{
		Pascal:      pascal(words),
		Camel:       camel(words),
		Snake:       snake(words),
		Plural:      pascal(plural),
		PluralCamel: camel(plural),
		PluralSnake: snake(plural),
		PluralKebab: strings.Join(plural, "-"),
		Module:      module,
	}
}

// splitWords breaks an identifier on separators *and* on camelCase humps, so
// every accepted input spelling normalises to the same word list.
func splitWords(s string) []string {
	var (
		words   []string
		current []rune
	)

	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToLower(string(current)))
			current = nil
		}
	}

	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '.':
			flush()
		case unicode.IsUpper(r):
			// Start a new word at a lower->upper hump, but keep acronyms
			// like "API" together until the hump back down ("APIKey").
			if i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) {
				flush()
			} else if i > 0 && i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1]) {
				flush()
			}
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}
	flush()

	return words
}

func pascal(words []string) string {
	var b strings.Builder
	for _, w := range words {
		b.WriteString(title(w))
	}
	return b.String()
}

func camel(words []string) string {
	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(w)
			continue
		}
		b.WriteString(title(w))
	}
	return b.String()
}

func snake(words []string) string { return strings.Join(words, "_") }

func title(w string) string {
	if w == "" {
		return w
	}
	// Preserve common initialisms the way Go style wants them.
	if upper, ok := initialisms[w]; ok {
		return upper
	}
	r := []rune(w)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}

var initialisms = map[string]string{
	"api": "API", "id": "ID", "url": "URL", "uri": "URI",
	"http": "HTTP", "json": "JSON", "sql": "SQL", "uuid": "UUID",
}

// pluralize covers the English rules that matter for table names. Irregular
// nouns ("person" -> "people") are not handled — pass the plural you want via
// the model's TableName if you hit one.
func pluralize(w string) string {
	switch {
	case w == "":
		return w
	case strings.HasSuffix(w, "y") && len(w) > 1 && !isVowel(rune(w[len(w)-2])):
		return w[:len(w)-1] + "ies"
	case hasAnySuffix(w, "s", "x", "z", "ch", "sh"):
		return w + "es"
	default:
		return w + "s"
	}
}

func isVowel(r rune) bool { return strings.ContainsRune("aeiou", unicode.ToLower(r)) }

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}
