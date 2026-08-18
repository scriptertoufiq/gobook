import type { Envelope, ListParams, Post } from '../types/api'
import { request, requestData } from './client'

const base = '/api/v1/posts'

function query(params: ListParams): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const encoded = search.toString()
  return encoded ? `?${encoded}` : ''
}

/**
 * One page of posts. The envelope is returned whole rather than just `data`,
 * because the caller needs `meta` to know whether another page exists.
 */
export async function listPosts(params: ListParams = {}): Promise<Envelope<Post[]>> {
  return request<Post[]>(`${base}${query(params)}`)
}

/**
 * The author is not a parameter: the API takes it from the access token and
 * ignores any user_id in the body, so there is nothing to send.
 */
export async function createPost(title: string, content: string): Promise<Post> {
  return requestData<Post>(base, { method: 'POST', body: { title, content } })
}

export async function getPost(id: number): Promise<Post> {
  return requestData<Post>(`${base}/${id}`)
}

export async function updatePost(
  id: number,
  payload: { title?: string; content?: string },
): Promise<Post> {
  return requestData<Post>(`${base}/${id}`, { method: 'PATCH', body: payload })
}

/** Only the author may delete; anyone else gets a 403. */
export async function deletePost(id: number): Promise<void> {
  await request(`${base}/${id}`, { method: 'DELETE' })
}
