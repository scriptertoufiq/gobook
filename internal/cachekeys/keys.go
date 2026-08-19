// Package cachekeys is the single source of truth for cache key formats.
//
// It exists so the code that reads a key and the code that clears it cannot
// drift apart — a stale entry caused by two slightly different key builders is
// the kind of bug that only shows up in production.
package cachekeys

import "github.com/scriptertoufiq/gobook/pkg/cache"

// Post is the key for a single post: "posts:show:<id>".
func Post(id uint) string {
	return cache.Key("posts", "show", id)
}

// ReactionCounts is the hash holding a post's tally by type.
func ReactionCounts(postID uint) string {
	return cache.Key("reactions", "count", postID)
}

// ReactionByUser is one person's reactions, a field per post.
//
// Keyed by user rather than by (post, user) pair, and holding only reactions
// they actually made. A key per pair would grow with views rather than with
// reactions — every reader of every post leaving a permanent trace — which is
// the difference between gigabytes and megabytes at scale.
func ReactionByUser(userID uint) string {
	return cache.Key("reactions", "byuser", userID)
}

// ReactionUserLoaded marks that a person's complete set of reactions is in
// Redis. It is what lets a missing field mean "they did not react" rather than
// "we have not looked", without storing anything per post they merely read.
func ReactionUserLoaded(userID uint) string {
	return cache.Key("reactions", "loaded", userID)
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
