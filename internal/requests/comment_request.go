package requests

// CreateCommentRequest carries only the text. Who is writing comes from the
// access token, and what it is attached to comes from the URL — the same
// reasoning that keeps user_id off CreatePostRequest.
//
// The upper bound is generous but present: without one, a single request can
// put a megabyte of text into a TEXT column and every reader of that thread
// then pays to download it.
type CreateCommentRequest struct {
	Body string `json:"body" binding:"required,min=1,max=5000"`
}

// UpdateCommentRequest uses a pointer so "field absent" and "field set to
// empty" stay distinguishable, matching the other update DTOs — though with a
// single field the distinction is mostly about consistency.
type UpdateCommentRequest struct {
	Body *string `json:"body" binding:"omitempty,min=1,max=5000"`
}
