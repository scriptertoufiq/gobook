// These mirror the Go types one-for-one. When a backend resource gains a
// field, add it here — that is what makes a response-shape change a compile
// error instead of an undefined at runtime.
//
//   Envelope      -> pkg/response.Envelope
//   Fault         -> pkg/response.Fault
//   User          -> internal/resources.UserResource
//   TokenPair     -> internal/resources.TokenResource
//   PaginationMeta-> pkg/pagination.Meta

export interface Envelope<T> {
  success: boolean
  message?: string
  data?: T
  meta?: PaginationMeta
  error?: Fault
}

export interface Fault {
  code: string
  message: string
  /** Per-field validation messages, keyed by the JSON field name. */
  fields?: Record<string, string>
}

export interface PaginationMeta {
  page: number
  per_page: number
  total: number
  last_page: number
}

export interface User {
  id: number
  name: string
  email: string
  role: 'user' | 'admin'
  is_active: boolean
  email_verified: boolean
  email_verified_at?: string
  created_at: string
  updated_at: string
}

/** The five responses a post accepts. Mirrors models.ReactionTypes. */
export type ReactionKey = 'like' | 'love' | 'care' | 'sad' | 'angry'

/** Mirrors internal/resources.ReactionResource. */
export interface Reactions {
  /** Keyed by type; types nobody has chosen are omitted. */
  counts: Partial<Record<ReactionKey, number>>
  total: number
  /** The viewer's own reaction, or null. */
  mine: ReactionKey | null
  /**
   * False when a replayed action was discarded for being older than what the
   * server already had. A queued entry that comes back false is settled and
   * should be dropped rather than retried.
   */
  applied: boolean
}

/** Mirrors internal/resources.CommentResource. */
export interface Comment {
  id: number
  post_id: number
  user_id: number
  /** null for a comment on a post; set for a reply. */
  parent_id: number | null
  body: string
  /** How many replies hang off this one. Always 0 on a reply. */
  reply_count: number
  edited: boolean
  created_at: string
  updated_at: string
}

/** Mirrors internal/resources.PostResource. */
export interface Post {
  id: number
  user_id: number
  title: string
  content: string
  created_at: string
  updated_at: string
  reactions: Reactions
  /** The whole conversation under this post, replies included. */
  comment_count: number
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_at: string
  expires_in: number
  user: User
}

export interface ListParams {
  page?: number
  per_page?: number
  search?: string
  sort_by?: string
  sort_dir?: 'asc' | 'desc'
  /** Posts only: narrow the listing to one author. */
  user_id?: number
}
