import { Link } from 'react-router-dom'
import { shortcuts } from '../../data/feed'
import { Avatar } from '../Avatar'
import {
  BookmarkIcon,
  ChevronDownIcon,
  ClockIcon,
  GroupsIcon,
  StoreIcon,
  VideoIcon,
} from '../icons'

const nav = [
  { label: 'Friends', icon: GroupsIcon, tone: 'text-sky-500' },
  { label: 'Memories', icon: ClockIcon, tone: 'text-blue-500' },
  { label: 'Saved', icon: BookmarkIcon, tone: 'text-violet-500' },
  { label: 'Groups', icon: GroupsIcon, tone: 'text-cyan-500' },
  { label: 'Reels', icon: VideoIcon, tone: 'text-rose-500' },
  { label: 'Marketplace', icon: StoreIcon, tone: 'text-amber-500' },
]

export function LeftRail({ name }: { name: string }) {
  return (
    <nav className="space-y-1 text-[15px]">
      <Link
        to="/account"
        className="flex items-center gap-3 rounded-lg px-2 py-2 font-medium transition
          hover:bg-slate-200/70 dark:hover:bg-slate-800"
      >
        <Avatar name={name} size={36} />
        <span className="truncate">{name}</span>
      </Link>

      {nav.map(({ label, icon: Icon, tone }) => (
        <button
          key={label}
          type="button"
          className="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left font-medium
            transition hover:bg-slate-200/70 dark:hover:bg-slate-800"
        >
          <span className={`flex h-9 w-9 items-center justify-center ${tone}`}>
            <Icon size={26} />
          </span>
          <span className="truncate">{label}</span>
        </button>
      ))}

      <button
        type="button"
        className="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left font-medium
          transition hover:bg-slate-200/70 dark:hover:bg-slate-800"
      >
        <span
          className="flex h-9 w-9 items-center justify-center rounded-full bg-slate-200
            text-slate-700 dark:bg-slate-800 dark:text-slate-300"
        >
          <ChevronDownIcon size={20} />
        </span>
        See more
      </button>

      <hr className="!my-3 border-slate-300 dark:border-slate-800" />

      <h2 className="px-2 pb-1 text-[17px] font-semibold text-slate-500 dark:text-slate-400">
        Your shortcuts
      </h2>

      {shortcuts.map((s) => (
        <button
          key={s.id}
          type="button"
          className="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left font-medium
            transition hover:bg-slate-200/70 dark:hover:bg-slate-800"
        >
          <Avatar name={s.name} size={36} />
          <span className="truncate">{s.name}</span>
        </button>
      ))}

      <p className="px-2 pt-4 text-xs leading-relaxed text-slate-500 dark:text-slate-500">
        Privacy · Terms · Advertising · Cookies · More · GoBook &copy; 2026
      </p>
    </nav>
  )
}
