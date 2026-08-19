import { useCallback, useEffect, useState } from 'react'
import { createComment, listComments } from '../../api/comments'
import { toFormError } from '../../lib/errors'
import type { Comment } from '../../types/api'
import { Alert } from '../ui'
import { CommentComposer } from './CommentComposer'
import { CommentItem } from './CommentItem'

const PER_PAGE = 10

/**
 * A post's conversation.
 *
 * Paged with a button rather than infinite scroll: the thread sits inside a
 * page that scrolls for its own reasons, and hijacking that to load comments
 * makes it impossible to reach anything below.
 */
export function CommentThread({
  postID,
  total,
  onChanged,
}: {
  postID: number
  total: number
  /**
   * Called whenever the conversation changes, so the page can re-read the
   * post's count. A delta is not passed because it is not always knowable —
   * deleting a comment removes its replies too.
   */
  onChanged: () => void
}) {
  const [comments, setComments] = useState<Comment[]>([])
  const [page, setPage] = useState(1)
  const [lastPage, setLastPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(
    async (target: number) => {
      setLoading(true)
      setError(null)

      try {
        const envelope = await listComments(postID, { page: target, per_page: PER_PAGE })
        const batch = envelope.data ?? []

        setComments((current) => {
          if (target === 1) return batch
          const seen = new Set(current.map((c) => c.id))
          return [...current, ...batch.filter((c) => !seen.has(c.id))]
        })
        setPage(envelope.meta?.page ?? target)
        setLastPage(envelope.meta?.last_page ?? target)
      } catch (err) {
        setError(toFormError(err).message)
      } finally {
        setLoading(false)
      }
    },
    [postID],
  )

  useEffect(() => {
    void load(1)
  }, [load])

  async function submit(body: string) {
    const created = await createComment(postID, body)
    setComments((current) => [...current, created])
    onChanged()
  }

  const hasMore = page < lastPage

  return (
    <section className="space-y-4">
      <h2 className="text-base font-semibold">
        {total === 0 ? 'Comments' : `${total} ${total === 1 ? 'comment' : 'comments'}`}
      </h2>

      <CommentComposer placeholder="Write a comment…" onSubmit={submit} />

      {error && (
        <Alert kind="error">
          <div className="flex items-center justify-between gap-3">
            <span>{error}</span>
            <button
              type="button"
              onClick={() => void load(page)}
              className="text-xs font-semibold underline"
            >
              Retry
            </button>
          </div>
        </Alert>
      )}

      {loading && comments.length === 0 && (
        <p className="text-sm text-slate-500 dark:text-slate-400">Loading the conversation…</p>
      )}

      {!loading && comments.length === 0 && !error && (
        <p className="text-sm text-slate-500 dark:text-slate-400">
          No comments yet. Be the first to say something.
        </p>
      )}

      <div className="space-y-4">
        {comments.map((comment) => (
          <CommentItem
            key={comment.id}
            comment={comment}
            onChanged={(updated) =>
              setComments((current) => current.map((c) => (c.id === updated.id ? updated : c)))
            }
            onDeleted={(id) => {
              // A top-level comment takes its replies with it, so the drop is
              // more than one — reload rather than guess.
              setComments((current) => current.filter((c) => c.id !== id))
              void load(1)
              onChanged()
            }}
            onReplyAdded={onChanged}
          />
        ))}
      </div>

      {hasMore && (
        <button
          type="button"
          disabled={loading}
          onClick={() => void load(page + 1)}
          className="text-sm font-semibold text-slate-600 hover:underline disabled:opacity-60
            dark:text-slate-400"
        >
          {loading ? 'Loading…' : 'View more comments'}
        </button>
      )}
    </section>
  )
}
