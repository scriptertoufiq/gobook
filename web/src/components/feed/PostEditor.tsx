import { useState, type FormEvent } from 'react'
import { updatePost } from '../../api/posts'
import { toFormError, type FormError } from '../../lib/errors'
import type { Post } from '../../types/api'
import { Alert } from '../ui'

/**
 * The edit form, shared by the feed card and the detail page so the two cannot
 * drift apart in validation, wording or behaviour.
 *
 * Only fields the caller actually changed are sent: the API's UpdatePostRequest
 * uses pointers precisely so an absent field means "leave it alone", and
 * sending everything back would discard that distinction.
 */
export function PostEditor({
  post,
  onSaved,
  onCancel,
}: {
  post: Post
  onSaved: (post: Post) => void
  onCancel: () => void
}) {
  const [title, setTitle] = useState(post.title)
  const [content, setContent] = useState(post.content)
  const [error, setError] = useState<FormError | null>(null)
  const [busy, setBusy] = useState(false)

  const changed = title !== post.title || content !== post.content

  async function submit(event: FormEvent) {
    event.preventDefault()

    if (!changed) {
      onCancel()
      return
    }

    setBusy(true)
    setError(null)

    try {
      const payload: { title?: string; content?: string } = {}
      if (title !== post.title) payload.title = title
      if (content !== post.content) payload.content = content

      onSaved(await updatePost(post.id, payload))
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      {error && <Alert kind="error">{error.message}</Alert>}

      <div>
        <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
          Title
        </label>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={200}
          required
          className={`w-full rounded-lg border px-3 py-2 text-[15px] font-medium outline-none
            transition focus:ring-2 focus:ring-blue-500/30 dark:bg-slate-950
            ${error?.fields.title ? 'border-red-400' : 'border-slate-300 focus:border-blue-500 dark:border-slate-700'}`}
        />
        {error?.fields.title && (
          <p className="mt-1 text-xs text-red-600 dark:text-red-400">{error.fields.title}</p>
        )}
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
          Content
        </label>
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          rows={5}
          required
          className={`w-full resize-y rounded-lg border px-3 py-2 text-[15px] outline-none
            transition focus:ring-2 focus:ring-blue-500/30 dark:bg-slate-950
            ${error?.fields.content ? 'border-red-400' : 'border-slate-300 focus:border-blue-500 dark:border-slate-700'}`}
        />
        {error?.fields.content && (
          <p className="mt-1 text-xs text-red-600 dark:text-red-400">{error.fields.content}</p>
        )}
      </div>

      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={busy}
          className="rounded-lg px-4 py-2 text-sm font-medium text-slate-600 transition
            hover:bg-slate-100 disabled:opacity-50 dark:text-slate-400 dark:hover:bg-slate-800"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={busy || title.trim() === '' || content.trim() === ''}
          className="rounded-lg bg-blue-600 px-5 py-2 text-sm font-semibold text-white transition
            hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-blue-600/40"
        >
          {busy ? 'Saving…' : changed ? 'Save changes' : 'Done'}
        </button>
      </div>
    </form>
  )
}
