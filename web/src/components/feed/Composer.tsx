import { useRef, useState, type FormEvent } from 'react'
import { createPost } from '../../api/posts'
import { toFormError, type FormError } from '../../lib/errors'
import type { Post } from '../../types/api'
import { Avatar } from '../Avatar'
import { Alert } from '../ui'
import { LiveIcon, PhotoIcon, SmileyIcon } from '../icons'

/**
 * The composer. Collapsed it is a button, not an input — expanding on click is
 * what lets the form carry a title field without the resting state looking like
 * a form. The API requires a title, so there is no single-box version of this.
 */
export function Composer({ name, onCreated }: { name: string; onCreated: (post: Post) => void }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [error, setError] = useState<FormError | null>(null)
  const [busy, setBusy] = useState(false)
  const titleRef = useRef<HTMLInputElement>(null)

  const firstName = name.split(' ')[0]

  function expand() {
    setOpen(true)
    // Focus after the field exists.
    requestAnimationFrame(() => titleRef.current?.focus())
  }

  function reset() {
    setOpen(false)
    setTitle('')
    setContent('')
    setError(null)
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)

    try {
      const created = await createPost(title, content)
      onCreated(created)
      reset()
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div className="flex items-center gap-3">
        <Avatar name={name} size={40} />

        {open ? (
          <span className="text-[15px] font-semibold">Create post</span>
        ) : (
          <button
            type="button"
            onClick={expand}
            className="flex-1 rounded-full bg-slate-100 px-4 py-2.5 text-left text-[15px]
              text-slate-500 transition hover:bg-slate-200
              dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
          >
            What&rsquo;s on your mind, {firstName}?
          </button>
        )}
      </div>

      {open ? (
        <form onSubmit={submit} className="mt-3 space-y-3">
          {error && <Alert kind="error">{error.message}</Alert>}

          <div>
            <input
              ref={titleRef}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Title"
              maxLength={200}
              required
              className={`w-full rounded-lg border px-3 py-2 text-[15px] font-medium outline-none
                transition focus:ring-2 focus:ring-blue-500/30 dark:bg-slate-950
                ${error?.fields.title ? 'border-red-400' : 'border-slate-300 focus:border-blue-500 dark:border-slate-700'}`}
            />
            {error?.fields.title && (
              <p className="mt-1 px-1 text-xs text-red-600 dark:text-red-400">{error.fields.title}</p>
            )}
          </div>

          <div>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={`What's on your mind, ${firstName}?`}
              rows={4}
              required
              className={`w-full resize-y rounded-lg border px-3 py-2 text-[15px] outline-none
                transition focus:ring-2 focus:ring-blue-500/30 dark:bg-slate-950
                ${error?.fields.content ? 'border-red-400' : 'border-slate-300 focus:border-blue-500 dark:border-slate-700'}`}
            />
            {error?.fields.content && (
              <p className="mt-1 px-1 text-xs text-red-600 dark:text-red-400">{error.fields.content}</p>
            )}
          </div>

          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1">
              <Decoration label="Go live" tone="text-rose-500">
                <LiveIcon size={22} />
              </Decoration>
              <Decoration label="Add photo" tone="text-emerald-500">
                <PhotoIcon size={22} />
              </Decoration>
              <Decoration label="Add feeling" tone="text-amber-500">
                <SmileyIcon size={22} />
              </Decoration>
            </div>

            <div className="flex gap-2">
              <button
                type="button"
                onClick={reset}
                disabled={busy}
                className="rounded-lg px-3 py-2 text-sm font-medium text-slate-600 transition
                  hover:bg-slate-100 disabled:opacity-50 dark:text-slate-400 dark:hover:bg-slate-800"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={busy || title.trim() === '' || content.trim() === ''}
                className="rounded-lg bg-blue-600 px-5 py-2 text-sm font-semibold text-white
                  transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-blue-600/40"
              >
                {busy ? 'Posting…' : 'Post'}
              </button>
            </div>
          </div>
        </form>
      ) : (
        <div className="mt-3 flex items-center justify-around border-t border-slate-200 pt-2 dark:border-slate-800">
          <Shortcut onClick={expand} label="Live video" tone="text-rose-500">
            <LiveIcon size={22} />
          </Shortcut>
          <Shortcut onClick={expand} label="Photo" tone="text-emerald-500">
            <PhotoIcon size={22} />
          </Shortcut>
          <Shortcut onClick={expand} label="Feeling" tone="text-amber-500">
            <SmileyIcon size={22} />
          </Shortcut>
        </div>
      )}
    </div>
  )
}

function Shortcut({
  children,
  label,
  tone,
  onClick,
}: {
  children: React.ReactNode
  label: string
  tone: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-1 items-center justify-center gap-2 rounded-lg py-2 text-sm
        font-medium text-slate-600 transition hover:bg-slate-100
        dark:text-slate-400 dark:hover:bg-slate-800"
    >
      <span className={tone}>{children}</span>
      {label}
    </button>
  )
}

// Decorative only — attachments are not implemented, so these do nothing rather
// than pretending to.
function Decoration({
  children,
  label,
  tone,
}: {
  children: React.ReactNode
  label: string
  tone: string
}) {
  return (
    <span aria-hidden="true" title={`${label} — not implemented yet`} className={`p-2 opacity-40 ${tone}`}>
      {children}
    </span>
  )
}
