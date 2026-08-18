import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/context'
import { Avatar } from './Avatar'
import { Brand } from './Brand'
import {
  BellIcon,
  ChatIcon,
  ChevronDownIcon,
  GridIcon,
  GroupsIcon,
  HomeIcon,
  SearchIcon,
  StoreIcon,
  VideoIcon,
} from './icons'

/**
 * Three-zone header: brand + search, centred primary nav, account actions.
 *
 * The centre nav collapses below `md` — five icon targets and a search field
 * cannot share a phone's width, and the primary nav is reachable from the feed
 * itself on those sizes.
 */
export function TopBar() {
  const { user, isAdmin, logout } = useAuth()
  const navigate = useNavigate()

  async function signOut() {
    await logout()
    navigate('/login', { replace: true })
  }

  const tab = ({ isActive }: { isActive: boolean }) =>
    `relative flex h-12 flex-1 items-center justify-center rounded-lg transition
     md:w-24 md:flex-none ${
       isActive
         ? 'text-blue-600 dark:text-blue-500'
         : 'text-slate-500 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800'
     }`

  return (
    <header
      className="sticky top-0 z-30 border-b border-slate-200 bg-white
        dark:border-slate-800 dark:bg-slate-900"
    >
      <div className="flex h-14 items-center justify-between gap-2 px-3">
        {/* Left: brand + search */}
        <div className="flex items-center gap-2 md:w-64">
          <NavLink to="/" aria-label="GoBook home">
            <Brand size={40} />
          </NavLink>
          <label className="relative hidden sm:block">
            <span className="sr-only">Search GoBook</span>
            <SearchIcon
              size={18}
              className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-slate-500"
            />
            <input
              type="search"
              placeholder="Search GoBook"
              className="w-40 rounded-full bg-slate-100 py-2 pr-3 pl-9 text-sm outline-none
                transition focus:ring-2 focus:ring-blue-500/40 lg:w-56
                dark:bg-slate-800 dark:placeholder:text-slate-500"
            />
          </label>
        </div>

        {/* Centre: primary nav */}
        <nav className="hidden flex-1 items-center justify-center gap-1 md:flex">
          <NavLink to="/" end className={tab} aria-label="Home">
            {({ isActive }) => (
              <>
                <HomeIcon size={26} filled={isActive} />
                {isActive && (
                  <span className="absolute inset-x-2 -bottom-px h-[3px] rounded-t bg-blue-600 dark:bg-blue-500" />
                )}
              </>
            )}
          </NavLink>
          <button type="button" aria-label="Watch" className={tab({ isActive: false })}>
            <VideoIcon size={26} />
          </button>
          {isAdmin ? (
            <NavLink to="/users" className={tab} aria-label="Users">
              {({ isActive }) => (
                <>
                  <GroupsIcon size={26} filled={isActive} />
                  {isActive && (
                    <span className="absolute inset-x-2 -bottom-px h-[3px] rounded-t bg-blue-600 dark:bg-blue-500" />
                  )}
                </>
              )}
            </NavLink>
          ) : (
            <button type="button" aria-label="Groups" className={tab({ isActive: false })}>
              <GroupsIcon size={26} />
            </button>
          )}
          <button type="button" aria-label="Marketplace" className={tab({ isActive: false })}>
            <StoreIcon size={26} />
          </button>
        </nav>

        {/* Right: account actions */}
        <div className="flex items-center gap-1.5 md:w-64 md:justify-end">
          <RoundButton label="Menu">
            <GridIcon size={20} />
          </RoundButton>
          <RoundButton label="Messages">
            <ChatIcon size={20} />
          </RoundButton>
          <RoundButton label="Notifications" badge={1}>
            <BellIcon size={20} />
          </RoundButton>

          <button
            type="button"
            onClick={() => void signOut()}
            title={`${user?.name} — sign out`}
            className="relative ml-0.5 rounded-full transition hover:opacity-80"
          >
            <Avatar name={user?.name ?? '?'} size={36} />
            <span
              className="absolute -right-0.5 -bottom-0.5 flex h-4 w-4 items-center justify-center
                rounded-full border border-white bg-slate-200 text-slate-700
                dark:border-slate-900 dark:bg-slate-700 dark:text-slate-200"
            >
              <ChevronDownIcon size={12} />
            </span>
          </button>
        </div>
      </div>
    </header>
  )
}

function RoundButton({
  children,
  label,
  badge,
}: {
  children: React.ReactNode
  label: string
  badge?: number
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className="relative hidden h-10 w-10 items-center justify-center rounded-full
        bg-slate-100 text-slate-700 transition hover:bg-slate-200
        sm:flex dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
    >
      {children}
      {badge && (
        <span
          className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center
            rounded-full bg-red-600 px-1 text-[10px] font-semibold text-white"
        >
          {badge}
        </span>
      )}
    </button>
  )
}
