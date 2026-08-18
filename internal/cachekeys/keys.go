// Package cachekeys is the single source of truth for cache key formats.
//
// It exists so the code that reads a key and the code that clears it cannot
// drift apart — a stale entry caused by two slightly different key builders is
// the kind of bug that only shows up in production.
package cachekeys

import "github.com/scriptertoufiq/go-mvc/pkg/cache"

// Post is the key for a single post: "posts:show:<id>".
func Post(id uint) string {
	return cache.Key("posts", "show", id)
}

// PostListPrefix covers every cached page of the post listing. Listings are
// keyed by page, search and sort, so they are cleared as a family rather than
// individually — see cache.DeleteByPrefix.
func PostListPrefix() string {
	return cache.Key("posts", "list")
}
