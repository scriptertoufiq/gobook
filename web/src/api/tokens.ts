// Token storage.
//
// localStorage is chosen deliberately and it is a trade-off worth naming: it
// survives a page reload (a sessionStorage-backed app logs you out on every
// refresh, which users read as a bug) but it is readable by any script on the
// page, so an XSS bug becomes a token theft. The backend limits the blast
// radius — access tokens live 15 minutes, and refresh tokens are single-use
// and rotate — but if you need cookie-based storage, this is the one file to
// change: swap these four functions for httpOnly cookies and have the Go side
// set them.

const ACCESS_KEY = 'auth.access_token'
const REFRESH_KEY = 'auth.refresh_token'

/** Notified when tokens are cleared out from under the app (refresh failed). */
type Listener = () => void
const listeners = new Set<Listener>()

export function onTokensCleared(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

export function setTokens(access: string, refresh: string): void {
  localStorage.setItem(ACCESS_KEY, access)
  localStorage.setItem(REFRESH_KEY, refresh)
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
  listeners.forEach((fn) => fn())
}
