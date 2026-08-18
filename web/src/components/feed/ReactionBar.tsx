import { useEffect, useRef, useState } from 'react'
import { reactionFor, reactions, type ReactionKey } from '../../lib/reactions'
import { getReaction, subscribe, toggleReaction } from '../../lib/reactionStore'
import { LikeIcon } from '../icons'

/**
 * The reaction control: a button showing your current choice, and a picker.
 *
 * Opening on click rather than hover alone — hover pickers are unreachable by
 * keyboard and unusable on touch. Hover still opens it on a pointer device,
 * because that is the interaction people expect from a feed.
 */
export function ReactionButton({ postID }: { postID: number }) {
  const [current, setCurrent] = useState<ReactionKey | null>(() => getReaction(postID))
  const [open, setOpen] = useState(false)
  const wrapper = useRef<HTMLDivElement>(null)
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Keep every card for this post in step — the feed and the detail page can
  // both be mounted across a navigation.
  useEffect(() => subscribe(() => setCurrent(getReaction(postID))), [postID])

  useEffect(() => {
    if (!open) return

    const onPointerDown = (e: MouseEvent) => {
      if (!wrapper.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }

    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  // Clear any pending close when the component goes away mid-hover.
  useEffect(() => () => { if (closeTimer.current) clearTimeout(closeTimer.current) }, [])

  const active = reactionFor(current)

  function choose(key: ReactionKey) {
    setCurrent(toggleReaction(postID, key))
    setOpen(false)
  }

  function openNow() {
    if (closeTimer.current) clearTimeout(closeTimer.current)
    setOpen(true)
  }

  function closeSoon() {
    // A grace period, so moving the pointer from the button to the picker does
    // not close it on the way.
    closeTimer.current = setTimeout(() => setOpen(false), 250)
  }

  return (
    <div
      ref={wrapper}
      className="relative flex-1"
      onMouseEnter={openNow}
      onMouseLeave={closeSoon}
    >
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={active ? `Your reaction: ${active.label}. Change or remove it.` : 'React to this post'}
        onClick={() => (active ? choose(active.key) : setOpen((v) => !v))}
        className={`flex w-full items-center justify-center gap-2 rounded-lg py-2 text-sm
          font-semibold transition hover:bg-slate-100 dark:hover:bg-slate-800
          ${active ? active.tone : 'text-slate-600 dark:text-slate-400'}`}
      >
        {active ? (
          <span aria-hidden="true" className="text-base leading-none">
            {active.emoji}
          </span>
        ) : (
          <LikeIcon />
        )}
        {active ? active.label : 'Like'}
      </button>

      {open && (
        <div
          role="menu"
          onMouseEnter={openNow}
          onMouseLeave={closeSoon}
          className="absolute bottom-full left-1/2 z-20 mb-2 flex -translate-x-1/2 gap-1
            rounded-full border border-slate-200 bg-white p-1.5 shadow-lg
            dark:border-slate-700 dark:bg-slate-800"
        >
          {reactions.map((r) => (
            <button
              key={r.key}
              type="button"
              role="menuitem"
              title={r.label}
              aria-label={r.label}
              aria-pressed={current === r.key}
              onClick={() => choose(r.key)}
              className={`rounded-full p-1.5 text-2xl leading-none transition
                hover:scale-125 hover:bg-slate-100 dark:hover:bg-slate-700
                ${current === r.key ? 'scale-110 bg-slate-100 dark:bg-slate-700' : ''}`}
            >
              <span aria-hidden="true">{r.emoji}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

/**
 * The summary line above the actions.
 *
 * It shows only reactions that actually exist — which, with no backend, means
 * only yours. Inventing a count would be a lie the UI cannot back up.
 */
export function ReactionSummary({ postID }: { postID: number }) {
  const [current, setCurrent] = useState<ReactionKey | null>(() => getReaction(postID))

  useEffect(() => subscribe(() => setCurrent(getReaction(postID))), [postID])

  const active = reactionFor(current)
  if (!active) return null

  return (
    <span className="flex items-center gap-1.5">
      <span
        className={`flex h-5 w-5 items-center justify-center rounded-full text-[11px]
          leading-none ring-2 ring-white dark:ring-slate-900 ${active.chip}`}
      >
        <span aria-hidden="true">{active.emoji}</span>
      </span>
      <span>You reacted &middot; {active.label}</span>
    </span>
  )
}
