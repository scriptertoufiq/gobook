import { ApiError } from '../api/client'

export interface FormError {
  message: string
  /** Per-field messages from a 422, keyed by JSON field name. */
  fields: Record<string, string>
}

/**
 * Normalises anything thrown by the client into something a form can render.
 * A 422 carries the field map the Go validation layer produced; everything
 * else collapses to a single message.
 */
export function toFormError(err: unknown): FormError {
  if (err instanceof ApiError) {
    return { message: err.message, fields: err.fields ?? {} }
  }
  if (err instanceof Error) {
    return { message: err.message, fields: {} }
  }
  return { message: 'Something went wrong. Please try again.', fields: {} }
}
