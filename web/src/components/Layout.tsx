import { Outlet } from 'react-router-dom'
import { TopBar } from './TopBar'

/**
 * The signed-in shell. It owns the header only — each page decides its own body
 * width, because the feed wants three full-bleed columns while Users and
 * Account want a narrow centred column.
 */
export function Layout() {
  return (
    <div className="min-h-screen bg-slate-100 dark:bg-slate-950">
      <TopBar />
      <Outlet />
    </div>
  )
}

/** Narrow centred body for the non-feed pages. */
export function PageBody({ children }: { children: React.ReactNode }) {
  return <div className="mx-auto max-w-5xl p-4 py-6">{children}</div>
}
