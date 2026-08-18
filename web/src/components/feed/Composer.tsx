import { Avatar } from '../Avatar'
import { LiveIcon, PhotoIcon, PlusIcon, SmileyIcon } from '../icons'

/**
 * The "what's on your mind" box. Static for now — the input is a button
 * rather than a real field, which is how the composer is normally built
 * anyway: clicking it opens a modal instead of typing inline.
 */
export function Composer({ name }: { name: string }) {
  const firstName = name.split(' ')[0]

  return (
    <div className="space-y-3">
      <Panel>
        <div className="flex items-center gap-3">
          <Avatar name={name} size={40} />
          <button
            type="button"
            className="flex-1 rounded-full bg-slate-100 px-4 py-2.5 text-left text-[15px]
              text-slate-500 transition hover:bg-slate-200
              dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
          >
            What&rsquo;s on your mind, {firstName}?
          </button>

          <div className="flex items-center gap-1">
            <IconButton label="Go live" tone="text-rose-500">
              <LiveIcon size={22} />
            </IconButton>
            <IconButton label="Add photo" tone="text-emerald-500">
              <PhotoIcon size={22} />
            </IconButton>
            <IconButton label="Add feeling" tone="text-amber-500">
              <SmileyIcon size={22} />
            </IconButton>
          </div>
        </div>
      </Panel>

      <Panel>
        <button type="button" className="flex w-full items-center gap-3 text-left">
          <span
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full
              bg-blue-600 text-white"
          >
            <PlusIcon size={20} />
          </span>
          <span>
            <span className="block text-[15px] font-semibold">Create story</span>
            <span className="block text-sm text-slate-500 dark:text-slate-400">
              Share a photo or write something
            </span>
          </span>
        </button>
      </Panel>
    </div>
  )
}

function Panel({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      {children}
    </div>
  )
}

function IconButton({
  children,
  label,
  tone,
}: {
  children: React.ReactNode
  label: string
  tone: string
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={`rounded-full p-2 transition hover:bg-slate-100 dark:hover:bg-slate-800 ${tone}`}
    >
      {children}
    </button>
  )
}
