import type { Comment, Envelope, ListParams } from '../types/api'
import { request, requestData } from './client'

function query(params: ListParams): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const encoded = search.toString()
  return encoded ? `?${encoded}` : ''
}

/**
 * A page of a post's top-level comments, oldest first.
 *
 * The envelope is returned whole because the caller needs `meta` to know
 * whether more of the conversation exists.
 */
export async function listComments(
  postID: number,
  params: ListParams = {},
): Promise<Envelope<Comment[]>> {
  return request<Comment[]>(`/api/v1/posts/${postID}/comments${query(params)}`)
}

/** A page of the replies under one comment. */
export async function listReplies(
  commentID: number,
  params: ListParams = {},
): Promise<Envelope<Comment[]>> {
  return request<Comment[]>(`/api/v1/comments/${commentID}/replies${query(params)}`)
}

/** Writes a comment on a post. The author comes from the token. */
export async function createComment(postID: number, body: string): Promise<Comment> {
  return requestData<Comment>(`/api/v1/posts/${postID}/comments`, {
    method: 'POST',
    body: { body },
  })
}

/**
 * Replies to a comment.
 *
 * Replying to a reply is allowed — the server attaches it to the same thread
 * rather than nesting a third level, so the returned comment's parent may not
 * be the id that was passed.
 */
export async function replyToComment(commentID: number, body: string): Promise<Comment> {
  return requestData<Comment>(`/api/v1/comments/${commentID}/replies`, {
    method: 'POST',
    body: { body },
  })
}

/** Edits a comment. Only its author may; anyone else gets a 403. */
export async function updateComment(commentID: number, body: string): Promise<Comment> {
  return requestData<Comment>(`/api/v1/comments/${commentID}`, {
    method: 'PATCH',
    body: { body },
  })
}

/** Deletes a comment. A top-level comment takes its replies with it. */
export async function deleteComment(commentID: number): Promise<void> {
  await request(`/api/v1/comments/${commentID}`, { method: 'DELETE' })
}
