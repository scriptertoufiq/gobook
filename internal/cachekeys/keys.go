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

// ReactionCounts is the hash holding a post's tally by type.
func ReactionCounts(postID uint) string {
	return cache.Key("reactions", "count", postID)
}

// ReactionUser is one person's current choice on one post.
func ReactionUser(postID, userID uint) string {
	return cache.Key("reactions", "user", postID, userID)
}

// ReactionHydrated marks that a post's counts have been loaded from MySQL.
// Without it an empty hash and an untouched post look identical.
func ReactionHydrated(postID uint) string {
	return cache.Key("reactions", "hydrated", postID)
}

// ReactionDirty is the set of pending (postID:userID) writes.
func ReactionDirty() string {
	return cache.Key("reactions", "dirty")
}

// ReactionFlushing is a batch claimed by the flusher. The run id keeps two
// concurrent claims from colliding, and makes an abandoned batch findable.
func ReactionFlushing(runID string) string {
	return cache.Key("reactions", "flushing", runID)
}

// ReactionFlushingPrefix finds batches abandoned by a crashed flusher.
func ReactionFlushingPrefix() string {
	return cache.Key("reactions", "flushing")
}

// PostListPrefix covers every cached page of the post listing. Listings are
// keyed by page, search and sort, so they are cleared as a family rather than
// individually — see cache.DeleteByPrefix.
func PostListPrefix() string {
	return cache.Key("posts", "list")
}
