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

/** Mirrors internal/resources.PostResource. */
export interface Post {
  id: number
  user_id: number
  title: string
  content: string
  created_at: string
  updated_at: string
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
