import type { TokenPair, User } from '../types/api'
import { request, requestData } from './client'
import { clearTokens, getRefreshToken, setTokens } from './tokens'

const base = '/api/v1/auth'

export async function login(email: string, password: string): Promise<TokenPair> {
  const pair = await requestData<TokenPair>(`${base}/login`, {
    method: 'POST',
    body: { email, password },
    anonymous: true,
  })
  setTokens(pair.access_token, pair.refresh_token)
  return pair
}

/**
 * Registration returns the created user, not a token pair — the backend does
 * not sign you in automatically. Callers log in afterwards if they want that.
 */
export async function register(name: string, email: string, password: string): Promise<User> {
  return requestData<User>(`${base}/register`, {
    method: 'POST',
    body: { name, email, password },
    anonymous: true,
  })
}

export async function me(): Promise<User> {
  return requestData<User>(`${base}/me`)
}

/**
 * Revokes the current refresh token, or every session when `everywhere` is set.
 * Local tokens are cleared regardless of what the server says — if the call
 * fails the user still expects to be signed out of this browser.
 */
export async function logout(everywhere = false): Promise<void> {
  const refresh = getRefreshToken()
  try {
    await request(`${base}/logout`, {
      method: 'POST',
      body: everywhere ? { all: true } : { refresh_token: refresh },
    })
  } finally {
    clearTokens()
  }
}

/** Always reports success — the backend will not confirm whether the address exists. */
export async function forgotPassword(email: string): Promise<string> {
  const envelope = await request<never>(`${base}/password/forgot`, {
    method: 'POST',
    body: { email },
    anonymous: true,
  })
  return envelope.message ?? 'If that email address has an account, a reset link is on its way.'
}

export async function resetPassword(token: string, password: string): Promise<string> {
  const envelope = await request<never>(`${base}/password/reset`, {
    method: 'POST',
    body: { token, password },
    anonymous: true,
  })
  return envelope.message ?? 'Password updated.'
}

/**
 * Changing a password revokes every session, so the backend hands back a fresh
 * pair to keep the caller signed in on this device. Storing it is not optional.
 */
export async function changePassword(current: string, next: string): Promise<TokenPair> {
  const pair = await requestData<TokenPair>(`${base}/password/change`, {
    method: 'POST',
    body: { current_password: current, password: next },
  })
  setTokens(pair.access_token, pair.refresh_token)
  return pair
}

export async function resendVerification(): Promise<string> {
  const envelope = await request<never>(`${base}/email/resend`, { method: 'POST' })
  return envelope.message ?? 'Verification email sent.'
}
