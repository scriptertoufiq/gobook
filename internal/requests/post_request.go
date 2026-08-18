package requests

// CreatePostRequest deliberately has no UserID field. The author is taken from
// the access token, so nobody can publish under someone else's name — the same
// reasoning that keeps `role` off RegisterRequest.
type CreatePostRequest struct {
	Title   string `json:"title" binding:"required,min=3,max=200"`
	Content string `json:"content" binding:"required,min=1"`
}

// UpdatePostRequest uses pointers so "field absent" and "field set to empty"
// stay distinguishable — a plain string could never express "leave title alone".
type UpdatePostRequest struct {
	Title   *string `json:"title" binding:"omitempty,min=3,max=200"`
	Content *string `json:"content" binding:"omitempty,min=1"`
}
