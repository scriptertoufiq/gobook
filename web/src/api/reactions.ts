import type { Reactions, ReactionKey } from '../types/api'
import { requestData } from './client'

const base = '/api/v1/posts'

/**
 * Sets a reaction. Sending the one already held takes it back — the server
 * treats a repeat as a toggle, so there is no separate "unreact this type".
 *
 * actedAt is sent only when replaying something queued offline. Leaving it out
 * means "now", which is every ordinary click. The server discards a replay
 * older than what it already has and says so through `applied`.
 */
export async function setReaction(
  postID: number,
  type: ReactionKey,
  actedAt?: string,
): Promise<Reactions> {
  return requestData<Reactions>(`${base}/${postID}/reaction`, {
    method: 'PUT',
    body: actedAt ? { type, acted_at: actedAt } : { type },
  })
}

/**
 * Removes whatever reaction the viewer holds. Idempotent — removing nothing
 * still succeeds.
 *
 * DELETE carries no body, so a replayed removal states its time in the query
 * string.
 */
export async function removeReaction(postID: number, actedAt?: string): Promise<Reactions> {
  const query = actedAt ? `?acted_at=${encodeURIComponent(actedAt)}` : ''
  return requestData<Reactions>(`${base}/${postID}/reaction${query}`, { method: 'DELETE' })
}

export async function getReactions(postID: number): Promise<Reactions> {
  return requestData<Reactions>(`${base}/${postID}/reactions`)
}

/** The zero state, for a post whose tally is not known yet. */
export function emptyReactions(): Reactions {
  return { counts: {}, total: 0, mine: null, applied: true }
}
