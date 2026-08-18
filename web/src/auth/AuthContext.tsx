import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import * as authApi from '../api/auth'
import { getAccessToken, onTokensCleared } from '../api/tokens'
import type { User } from '../types/api'
import { AuthContext, type AuthState } from './context'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  // On first paint, a stored token is only a *claim* that we are signed in.
  // /auth/me is what settles it — the token may have expired while the tab was
  // closed, or the account may have been disabled.
  useEffect(() => {
    let cancelled = false

    async function bootstrap() {
      if (!getAccessToken()) {
        setLoading(false)
        return
      }
      try {
        const current = await authApi.me()
        if (!cancelled) setUser(current)
      } catch {
        if (!cancelled) setUser(null)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void bootstrap()
    return () => {
      cancelled = true
    }
  }, [])

  // The API client clears tokens when a refresh fails. That happens deep inside
  // a fetch with no route context, so it reports back here rather than
  // navigating on its own.
  useEffect(() => onTokensCleared(() => setUser(null)), [])

  const login = useCallback(async (email: string, password: string) => {
    const pair = await authApi.login(email, password)
    setUser(pair.user)
  }, [])

  const logout = useCallback(async (everywhere = false) => {
    await authApi.logout(everywhere)
    setUser(null)
  }, [])

  const reload = useCallback(async () => {
    setUser(await authApi.me())
  }, [])

  const value = useMemo<AuthState>(
    () => ({ user, loading, login, logout, reload, isAdmin: user?.role === 'admin' }),
    [user, loading, login, logout, reload],
  )

  return <AuthContext value={value}>{children}</AuthContext>
}
