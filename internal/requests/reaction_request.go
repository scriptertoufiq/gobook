package requests

// SetReactionRequest carries only the reaction. Who is reacting comes from the
// access token and which post from the URL, so neither is expressible here —
// the same reasoning that keeps user_id off CreatePostRequest.
type SetReactionRequest struct {
	Type string `json:"type" binding:"required,oneof=like love care sad angry"`
}
