import { useEffect, useRef, useState } from 'react'
import { removeReaction, setReaction } from '../../api/reactions'
import { reactionFor, reactions } from '../../lib/reactions'
import { dequeue, enqueue, isTransportFailure, queued, subscribe } from '../../lib/reactionStore'
import type { Reactions, ReactionKey } from '../../types/api'
import { LikeIcon } from '../icons'

/**
 * The reaction control.
 *
 * The tally comes from the post payload; a click goes to the server and the
 * response replaces it. The UI updates first and reconciles after, so the
 * button never feels like it is waiting on a round trip.
 *
 * When the request cannot reach the server the click is parked in the outbox
 * and the optimistic state stays on screen, marked as pending — the person
 * reacted, and as far as they are concerned it happened.
 */
export function ReactionButton({
  postID,
  value,
  onChange,
}: {
  postID: number
  value: Reactions
  onChange: (next: Reactions) => void
}) {
  const [open, setOpen] = useState(false)
  const [waiting, setWaiting] = useState(false)
  const [isQueued, setIsQueued] = useState(() => queued(postID) !== null)

  const wrapper = useRef<HTMLDivElement>(null)
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // The outbox drains on its own; when this post's entry clears, drop the badge.
  useEffect(() => subscribe(() => setIsQueued(queued(postID) !== null)), [postID])

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

  useEffect(() => () => { if (closeTimer.current) clearTimeout(closeTimer.current) }, [])

  const active = reactionFor(value.mine)

  /** What the tally looks like with this change applied, before the server answers. */
  function predict(next: ReactionKey | null): Reactions {
    const counts = { ...value.counts }
    const previous = value.mine

    if (previous) counts[previous] = Math.max((counts[previous] ?? 1) - 1, 0)
    if (next) counts[next] = (counts[next] ?? 0) + 1

    for (const key of Object.keys(counts) as ReactionKey[]) {
      if (!counts[key]) delete counts[key]
    }

    const total = Object.values(counts).reduce((sum, n) => sum + (n ?? 0), 0)
    return { counts, total, mine: next, applied: true }
  }

  async function choose(key: ReactionKey) {
    setOpen(false)

    // Tapping the held reaction takes it back.
    const next = value.mine === key ? null : key

    onChange(predict(next))
    setWaiting(true)

    try {
      const settled = next === null ? await removeReaction(postID) : await setReaction(postID, next)

      onChange(settled)
      dequeue(postID) // a delivered click supersedes anything parked for this post
      setIsQueued(false)
    } catch (err) {
      if (isTransportFailure(err)) {
        // Never reached the server. Keep what the person sees and deliver later.
        enqueue(postID, next)
        setIsQueued(true)
      } else {
        // Refused. Put the tally back to what the server last told us.
        onChange(value)
      }
    } finally {
      setWaiting(false)
    }
  }

  function openNow() {
    if (closeTimer.current) clearTimeout(closeTimer.current)
    setOpen(true)
  }

  function closeSoon() {
    closeTimer.current = setTimeout(() => setOpen(false), 250)
  }

  return (
    <div ref={wrapper} className="relative flex-1" onMouseEnter={openNow} onMouseLeave={closeSoon}>
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={waiting}
        aria-label={active ? `Your reaction: ${active.label}. Change or remove it.` : 'React to this post'}
        onClick={() => (active ? void choose(active.key) : setOpen((v) => !v))}
        className={`flex w-full items-center justify-center gap-2 rounded-lg py-2 text-sm
          font-semibold transition hover:bg-slate-100 disabled:opacity-60
          dark:hover:bg-slate-800
          ${active ? active.tone : 'text-slate-600 dark:text-slate-400'}`}
      >
        {active ? (
          <span aria-hidden="true" className="text-base leading-none">{active.emoji}</span>
        ) : (
          <LikeIcon />
        )}
        {active ? active.label : 'Like'}
        {isQueued && (
          <span
            title="Saved on this device — will sync when the connection returns"
            className="ml-0.5 h-1.5 w-1.5 rounded-full bg-amber-500"
            aria-label="waiting to sync"
          />
        )}
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
              aria-pressed={value.mine === r.key}
              onClick={() => void choose(r.key)}
              className={`rounded-full p-1.5 text-2xl leading-none transition
                hover:scale-125 hover:bg-slate-100 dark:hover:bg-slate-700
                ${value.mine === r.key ? 'scale-110 bg-slate-100 dark:bg-slate-700' : ''}`}
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
 * The tally above the actions: which reactions a post has, and how many people
 * in total. Renders nothing when nobody has reacted.
 */
export function ReactionSummary({ value }: { value: Reactions }) {
  const present = reactions.filter((r) => (value.counts[r.key] ?? 0) > 0)
  if (present.length === 0) return null

  return (
    <span className="flex items-center gap-1.5">
      <span className="flex -space-x-1">
        {present.map((r) => (
          <span
            key={r.key}
            title={`${value.counts[r.key]} ${r.label}`}
            className={`flex h-5 w-5 items-center justify-center rounded-full text-[11px]
              leading-none ring-2 ring-white dark:ring-slate-900 ${r.chip}`}
          >
            <span aria-hidden="true">{r.emoji}</span>
          </span>
        ))}
      </span>
      <span className="tabular-nums">{value.total.toLocaleString()}</span>
      {value.mine && <span className="opacity-70">&middot; including you</span>}
    </span>
  )
}
