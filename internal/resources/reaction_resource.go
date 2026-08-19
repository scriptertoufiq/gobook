package resources

import (
	"github.com/scriptertoufiq/go-mvc/internal/models"
	"github.com/scriptertoufiq/go-mvc/internal/services"
)

// ReactionResource is how a post's reactions appear on the wire.
//
// Total is summed rather than stored. Keeping a separate running total would
// be a second counter to hold in step with the first, and two counters
// maintained independently eventually disagree; adding up five integers cannot
// drift from the numbers it is adding.
type ReactionResource struct {
	// Counts is keyed by reaction type. Types nobody has chosen are omitted.
	Counts map[string]int64 `json:"counts"`
	Total  int64            `json:"total"`
	// Mine is the viewer's own reaction, or null.
	Mine *string `json:"mine"`
}

func NewReactionResource(summary services.Summary) ReactionResource {
	counts := summary.Counts
	if counts == nil {
		counts = map[string]int64{}
	}

	resource := ReactionResource{Counts: counts, Total: summary.Total}
	if summary.Mine != "" {
		mine := summary.Mine
		resource.Mine = &mine
	}
	return resource
}

// EmptyReactions is what a post carries before anything is known about it —
// an explicit zero state, so a client never has to handle a missing key.
func EmptyReactions() ReactionResource {
	return ReactionResource{Counts: map[string]int64{}, Total: 0}
}

// ReactionTypes exposes the accepted set, so a client can render a picker
// without hardcoding the list a second time.
func ReactionTypes() []string { return models.ReactionTypes }
