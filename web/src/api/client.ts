import type { Envelope, Fault } from '../types/api'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from './tokens'

/** Same-origin in dev thanks to the Vite proxy; set VITE_API_URL to point elsewhere. */
const BASE = import.meta.env.VITE_API_URL ?? ''

/**
 * A failed request, carrying the backend's structured fault.
 *
 * `fields` is the per-field validation map the Go side returns on a 422, so a
 * form can highlight the offending inputs instead of showing one flat string.
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields?: Record<string, string>

  constructor(status: number, fault: Fault) {
    super(fault.message)
    this.name = 'ApiError'
    this.status = status
    this.code = fault.code
    this.fields = fault.fields
  }

  get isValidation(): boolean {
    return this.code === 'validation_failed'
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  /** Skip the Authorization header and the refresh-retry (used by auth endpoints). */
  anonymous?: boolean
  signal?: AbortSignal
}

/**
 * The in-flight refresh, shared by every caller.
 *
 * This single-flight is not an optimisation — it is required for correctness.
 * The backend rotates refresh tokens atomically: the revoking UPDATE is the
 * critical section, so of N concurrent exchanges of the same token exactly one
 * wins and the rest get a 401. If three requests 401 at once and each fired
 * its own refresh, two would lose that race and log the user out even though
 * their session is perfectly valid. So the first 401 starts the refresh and
 * everyone else awaits the same promise.
 */
let refreshInFlight: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  const refresh = getRefreshToken()
  if (!refresh) return false

  const response = await fetch(`${BASE}/api/v1/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refresh }),
  })

  if (!response.ok) {
    // The refresh token is spent, expired or revoked. Nothing left to try.
    clearTokens()
    return false
  }

  const envelope = (await response.json()) as Envelope<{
    access_token: string
    refresh_token: string
  }>

  if (!envelope.data) {
    clearTokens()
    return false
  }

  setTokens(envelope.data.access_token, envelope.data.refresh_token)
  return true
}

function ensureRefreshed(): Promise<boolean> {
  refreshInFlight ??= refreshTokens().finally(() => {
    refreshInFlight = null
  })
  return refreshInFlight
}

async function send(path: string, options: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = {}

  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  if (!options.anonymous) {
    const token = getAccessToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }

  return fetch(`${BASE}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    signal: options.signal,
  })
}

/**
 * Performs a request and unwraps the standard envelope.
 *
 * On a 401 it transparently refreshes once and replays the request, so a
 * 15-minute access token expiring mid-session is invisible to the caller.
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<Envelope<T>> {
  let response = await send(path, options)

  if (response.status === 401 && !options.anonymous && getRefreshToken()) {
    if (await ensureRefreshed()) {
      response = await send(path, options)
    }
  }

  // 204 has no body to parse.
  if (response.status === 204) {
    return { success: true } as Envelope<T>
  }

  let envelope: Envelope<T>
  try {
    envelope = (await response.json()) as Envelope<T>
  } catch {
    throw new ApiError(response.status, {
      code: 'unreadable_response',
      message: `The server returned a response that could not be read (HTTP ${response.status}).`,
    })
  }

  if (!response.ok || envelope.success === false) {
    throw new ApiError(
      response.status,
      envelope.error ?? { code: 'unknown_error', message: 'Something went wrong.' },
    )
  }

  return envelope
}

/** Convenience wrapper for the common case: return `data`, throw on anything else. */
export async function requestData<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const envelope = await request<T>(path, options)
  if (envelope.data === undefined) {
    throw new ApiError(200, { code: 'empty_response', message: 'The server returned no data.' })
  }
  return envelope.data
}
