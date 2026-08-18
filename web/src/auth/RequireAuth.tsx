import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from './context'

/**
 * Route guard. Mirrors the backend's two-step authorisation: `Auth` proves who
 * you are, `RequireRole` proves you may act here. This is a convenience for the
 * user — the server enforces both regardless, so a tampered client gains
 * nothing but a different error page.
 */
export function RequireAuth({ role }: { role?: 'admin' }) {
  const { user, loading } = useAuth()
  const location = useLocation()

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <span className="text-sm text-slate-500 dark:text-slate-400">Loading…</span>
      </div>
    )
  }

  if (!user) {
    // Remember where they were headed so login can send them back.
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  if (role && user.role !== role) {
    return <Navigate to="/" replace />
  }

  return <Outlet />
}
