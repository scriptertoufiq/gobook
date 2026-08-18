import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/context'
import { Button } from './ui'

export function Layout() {
  const { user, isAdmin, logout } = useAuth()
  const navigate = useNavigate()

  const link = ({ isActive }: { isActive: boolean }) =>
    `rounded-lg px-3 py-1.5 text-sm font-medium transition ${
      isActive
        ? 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-100'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800'
    }`

  async function signOut() {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-1">
            <span className="mr-3 font-semibold">GoBook</span>
            <NavLink to="/" end className={link}>
              Account
            </NavLink>
            {isAdmin && (
              <NavLink to="/users" className={link}>
                Users
              </NavLink>
            )}
          </div>

          <div className="flex items-center gap-3">
            <span className="hidden text-sm text-slate-500 sm:inline dark:text-slate-400">
              {user?.email}
            </span>
            <Button variant="ghost" onClick={() => void signOut()}>
              Sign out
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl p-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
