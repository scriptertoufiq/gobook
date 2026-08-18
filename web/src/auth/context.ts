import { createContext, use } from 'react'
import type { User } from '../types/api'

export interface AuthState {
  user: User | null
  /** True while the stored token is being validated on first paint. */
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  logout: (everywhere?: boolean) => Promise<void>
  /** Re-reads /auth/me — call after anything that changes the user's own record. */
  reload: () => Promise<void>
  isAdmin: boolean
}

export const AuthContext = createContext<AuthState | null>(null)

export function useAuth(): AuthState {
  const state = use(AuthContext)
  if (!state) {
    throw new Error('useAuth must be used inside <AuthProvider>')
  }
  return state
}
