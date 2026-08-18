/**
 * Inline icon set. Every icon inherits `currentColor` and sizes off the `size`
 * prop, so colour and scale are decided by the caller's text classes.
 */
type IconProps = { size?: number; className?: string; filled?: boolean }

const base = (size: number, className?: string) => ({
  width: size,
  height: size,
  viewBox: '0 0 24 24',
  className,
  'aria-hidden': true as const,
  xmlns: 'http://www.w3.org/2000/svg',
})

const stroke = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.8,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

export function HomeIcon({ size = 24, className, filled }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path
        d="M3.5 10.5 12 3.5l8.5 7M5.5 9.5v9a1 1 0 0 0 1 1H10v-5h4v5h3.5a1 1 0 0 0 1-1v-9"
        {...stroke}
        fill={filled ? 'currentColor' : 'none'}
      />
    </svg>
  )
}

export function VideoIcon({ size = 24, className, filled }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <rect x="3" y="5.5" width="13" height="13" rx="3" {...stroke} fill={filled ? 'currentColor' : 'none'} />
      <path d="m16 10.5 5-3v9l-5-3" {...stroke} fill={filled ? 'currentColor' : 'none'} />
    </svg>
  )
}

export function GroupsIcon({ size = 24, className, filled }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="9" cy="9" r="3.2" {...stroke} fill={filled ? 'currentColor' : 'none'} />
      <path d="M3.5 19c0-3 2.5-5 5.5-5s5.5 2 5.5 5" {...stroke} fill={filled ? 'currentColor' : 'none'} />
      <path d="M16 6.5a3 3 0 0 1 0 6M17.5 19c0-2.2-.8-3.8-2-4.7" {...stroke} />
    </svg>
  )
}

export function StoreIcon({ size = 24, className, filled }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M4 9.5h16v9a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1z" {...stroke} fill={filled ? 'currentColor' : 'none'} />
      <path d="M3.5 9.5 5 4.5h14l1.5 5a2.5 2.5 0 0 1-4.25 1.8 2.5 2.5 0 0 1-4.25 0 2.5 2.5 0 0 1-4.25 0A2.5 2.5 0 0 1 3.5 9.5Z" {...stroke} />
    </svg>
  )
}

export function BookmarkIcon({ size = 24, className, filled }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M6.5 4.5h11a1 1 0 0 1 1 1v14l-6.5-4-6.5 4v-14a1 1 0 0 1 1-1Z" {...stroke} fill={filled ? 'currentColor' : 'none'} />
    </svg>
  )
}

export function ClockIcon({ size = 24, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="12" cy="12" r="8.5" {...stroke} />
      <path d="M12 7.5V12l3 2" {...stroke} />
    </svg>
  )
}

export function SearchIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="11" cy="11" r="6.5" {...stroke} />
      <path d="m16 16 4 4" {...stroke} />
    </svg>
  )
}

export function BellIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M6.5 10a5.5 5.5 0 0 1 11 0c0 4 1.5 5.5 1.5 5.5H5S6.5 14 6.5 10Z" {...stroke} />
      <path d="M10 18.5a2 2 0 0 0 4 0" {...stroke} />
    </svg>
  )
}

export function ChatIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M4 11.5c0-3.9 3.6-7 8-7s8 3.1 8 7-3.6 7-8 7a9.6 9.6 0 0 1-2.6-.35L5.5 20l.9-3.1A6.8 6.8 0 0 1 4 11.5Z" {...stroke} />
    </svg>
  )
}

export function GridIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      {[6, 12, 18].flatMap((y) => [6, 12, 18].map((x) => <circle key={`${x}-${y}`} cx={x} cy={y} r="1.6" fill="currentColor" />))}
    </svg>
  )
}

export function ChevronDownIcon({ size = 18, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="m7 10 5 5 5-5" {...stroke} />
    </svg>
  )
}

export function DotsIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="6" cy="12" r="1.7" fill="currentColor" />
      <circle cx="12" cy="12" r="1.7" fill="currentColor" />
      <circle cx="18" cy="12" r="1.7" fill="currentColor" />
    </svg>
  )
}

export function CloseIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="m7 7 10 10M17 7 7 17" {...stroke} />
    </svg>
  )
}

export function PlusIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M12 5.5v13M5.5 12h13" {...stroke} />
    </svg>
  )
}

export function LikeIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M7 10.5 11 4a2 2 0 0 1 2.8 2.4L13 10h4.6a2 2 0 0 1 2 2.5l-1.4 5.5a2 2 0 0 1-2 1.5H7Z" {...stroke} />
      <path d="M7 10.5v9H5a1 1 0 0 1-1-1v-7a1 1 0 0 1 1-1Z" {...stroke} />
    </svg>
  )
}

export function CommentIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M20 11.8c0 3.8-3.6 6.9-8 6.9a9.6 9.6 0 0 1-2.6-.35L5.5 20l.9-3.1A6.7 6.7 0 0 1 4 11.8C4 8 7.6 4.9 12 4.9s8 3.1 8 6.9Z" {...stroke} />
    </svg>
  )
}

export function ShareIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M12 4.5 19 11h-4v3.5c0 2.8-2.4 5-5.5 5H8" {...stroke} />
      <path d="M12 4.5V11H9c-2.8 0-5 2.2-5 5v1.5" {...stroke} />
    </svg>
  )
}

export function PhotoIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <rect x="3.5" y="5" width="17" height="14" rx="2.5" {...stroke} />
      <circle cx="9" cy="10" r="1.6" {...stroke} />
      <path d="m4.5 17 4.5-4.5 3.5 3.5 3-2.5 4 3.5" {...stroke} />
    </svg>
  )
}

export function LiveIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="12" cy="12" r="3" {...stroke} />
      <path d="M7.5 7.5a6.4 6.4 0 0 0 0 9M16.5 16.5a6.4 6.4 0 0 0 0-9" {...stroke} />
    </svg>
  )
}

export function SmileyIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="12" cy="12" r="8.5" {...stroke} />
      <circle cx="9.3" cy="10" r="1" fill="currentColor" />
      <circle cx="14.7" cy="10" r="1" fill="currentColor" />
      <path d="M8.5 14a4.3 4.3 0 0 0 7 0" {...stroke} />
    </svg>
  )
}

export function PencilIcon({ size = 20, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <path d="M4.5 19.5h3l10-10a2.1 2.1 0 0 0-3-3l-10 10Z" {...stroke} />
    </svg>
  )
}

export function GlobeIcon({ size = 14, className }: IconProps) {
  return (
    <svg {...base(size, className)}>
      <circle cx="12" cy="12" r="8.5" {...stroke} />
      <path d="M3.5 12h17M12 3.5c4 4.5 4 12.5 0 17-4-4.5-4-12.5 0-17Z" {...stroke} />
    </svg>
  )
}
