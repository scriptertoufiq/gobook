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

// CommentGeneration counts how many times a post's conversation has changed.
//
// One counter per post, not one global: commenting on post 5 must not throw
// away everything cached for post 9. Bumping it orphans every cached page of
// that post's comments and replies at once — including the top-level pages,
// whose reply counts a new reply also changes.
func CommentGeneration(postID uint) string {
	return cache.Key("comments", "gen", postID)
}

// Comment names one cached comment.
//
// Not generation-versioned like the pages are, because a single comment can be
// invalidated precisely — the change event carries its id. The pages cannot, so
// they get the generation instead.
func Comment(commentID uint) string {
	return cache.Key("comments", "show", commentID)
}

// CommentPage names one cached page of a post's top-level comments.
//
// The generation is part of the key rather than something to look up and
// compare: a stale page is not found rather than found-and-rejected.
func CommentPage(postID uint, generation int64, page, perPage int, sortDir, search string) string {
	return cache.Key("comments", "post", postID, generation, page, perPage, sortDir, search)
}

// ReplyPage names one cached page of the replies under a comment. It shares the
// post's generation, so any change to the conversation clears both.
func ReplyPage(postID, parentID uint, generation int64, page, perPage int, sortDir, search string) string {
	return cache.Key("comments", "replies", parentID, generation, page, perPage, sortDir, search)
}

// PostListGeneration counts how many times the post listings have been
// invalidated.
//
// Cached listing pages embed its current value, so a write bumps this one
// counter and every page cached under the old number is orphaned at once —
// no scan, and the same cost whether the cache holds ten keys or ten million.
func PostListGeneration() string {
	return cache.Key("posts", "list", "gen")
}

// PostListPrefix covers every cached page of the post listing. Listings are
// keyed by page, search and sort, so they are cleared as a family rather than
// individually — see cache.DeleteByPrefix.
func PostListPrefix() string {
	return cache.Key("posts", "list")
}
