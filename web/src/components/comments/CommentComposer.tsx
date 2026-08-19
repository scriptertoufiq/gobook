import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useAuth } from '../../auth/context'
import { toFormError, type FormError } from '../../lib/errors'
import { Avatar } from '../Avatar'
import { Alert } from '../ui'

/**
 * The box for writing a comment or a reply.
 *
 * A textarea that grows with its content rather than a fixed box with a
 * scrollbar — a comment is usually one or two lines, and reserving five is
 * wasted space above every thread.
 */
export function CommentComposer({
  placeholder,
  submitLabel = 'Comment',
  initialValue = '',
  autoFocus = false,
  compact = false,
  showAvatar = true,
  onSubmit,
  onCancel,
}: {
  placeholder: string
  submitLabel?: string
  /** Text to start from. Editing passes the comment being changed. */
  initialValue?: string
  autoFocus?: boolean
  compact?: boolean
  /**
   * Whether to draw the writer's avatar. Off when the composer replaces a
   * comment's body in place, since that row already has one.
   */
  showAvatar?: boolean
  onSubmit: (body: string) => Promise<void>
  onCancel?: () => void
}) {
  const { user } = useAuth()
  const [body, setBody] = useState(initialValue)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<FormError | null>(null)
  const field = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (!autoFocus) return

    const node = field.current
    if (!node) return

    node.focus()
    // Cursor at the end rather than the start, so editing continues the
    // sentence instead of typing in front of it.
    node.setSelectionRange(node.value.length, node.value.length)
  }, [autoFocus])

  // Grow to fit, so a long comment is readable while being written.
  useEffect(() => {
    const node = field.current
    if (!node) return
    node.style.height = 'auto'
    node.style.height = `${node.scrollHeight}px`
  }, [body])

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (body.trim() === '') return

    setBusy(true)
    setError(null)

    try {
      await onSubmit(body.trim())
      // Back to where it started: empty for a new comment, and for an edit the
      // original text, which matters only if the form outlives the save.
      setBody(initialValue)
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  // Enter sends, Shift+Enter breaks the line — what a chat-shaped box is
  // expected to do. The button stays for anyone who does not know that.
  function onKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      void submit(event as unknown as FormEvent)
    }
  }

  return (
    <form onSubmit={submit} className="flex gap-2">
      {showAvatar && <Avatar name={user?.name ?? '?'} size={compact ? 28 : 34} />}

      <div className="min-w-0 flex-1 space-y-2">
        {error && <Alert kind="error">{error.message}</Alert>}

        <textarea
          ref={field}
          value={body}
          rows={1}
          maxLength={5000}
          placeholder={placeholder}
          onChange={(e) => setBody(e.target.value)}
          onKeyDown={onKeyDown}
          className={`w-full resize-none overflow-hidden rounded-2xl border px-3 py-2
            outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-500/25
            dark:bg-slate-950
            ${error?.fields.body ? 'border-red-400' : 'border-slate-300 dark:border-slate-700'}
            ${compact ? 'text-sm' : 'text-[15px]'}`}
        />
        {error?.fields.body && (
          <p className="px-1 text-xs text-red-600 dark:text-red-400">{error.fields.body}</p>
        )}

        {(body.trim() !== '' || onCancel) && (
          <div className="flex items-center justify-end gap-2">
            {onCancel && (
              <button
                type="button"
                onClick={onCancel}
                disabled={busy}
                className="rounded-lg px-3 py-1.5 text-sm font-medium text-slate-600 transition
                  hover:bg-slate-100 disabled:opacity-50 dark:text-slate-400 dark:hover:bg-slate-800"
              >
                Cancel
              </button>
            )}
            <button
              type="submit"
              disabled={busy || body.trim() === ''}
              className="rounded-lg bg-blue-600 px-4 py-1.5 text-sm font-semibold text-white
                transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-blue-600/40"
            >
              {busy ? 'Posting…' : submitLabel}
            </button>
          </div>
        )}
      </div>
    </form>
  )
}
