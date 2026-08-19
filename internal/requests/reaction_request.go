package requests

import "time"

// SetReactionRequest carries the reaction, and optionally when it was made.
//
// Who is reacting comes from the access token and which post from the URL, so
// neither is expressible here — the same reasoning that keeps user_id off
// CreatePostRequest.
type SetReactionRequest struct {
	Type string `json:"type" binding:"required,oneof=like love care sad angry"`

	// ActedAt is sent only by a client replaying reactions it queued while
	// offline. Absent means now, which is every ordinary request. The server
	// discards a replay older than the reaction already stored, and clamps a
	// time in the future, so a wrong device clock cannot win every conflict.
	ActedAt *time.Time `json:"acted_at" binding:"omitempty"`
}

// When returns the action time, or the zero time meaning "now".
func (r SetReactionRequest) When() time.Time {
	if r.ActedAt == nil {
		return time.Time{}
	}
	return *r.ActedAt
}
