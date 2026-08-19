import { useState } from 'react'
import { deleteComment, listReplies, replyToComment, updateComment } from '../../api/comments'
import { useAuth } from '../../auth/context'
import { toFormError } from '../../lib/errors'
import { relativeTime } from '../../lib/time'
import type { Comment } from '../../types/api'
import { Avatar } from '../Avatar'
import { ConfirmDialog } from '../ConfirmDialog'
import { Alert } from '../ui'
import { CommentComposer } from './CommentComposer'

const REPLIES_PER_PAGE = 10

/**
 * One comment, with its replies underneath.
 *
 * Replies are fetched only when asked for. A post can carry a hundred comments
 * of which two have long arguments under them; loading every thread up front
 * would pay for all of them to show none.
 */
export function CommentItem({
  comment,
  onChanged,
  onDeleted,
  onReplyAdded,
}: {
  comment: Comment
  onChanged: (updated: Comment) => void
  onDeleted: (id: number) => void
  onReplyAdded: () => void
}) {
  const { user } = useAuth()

  const [editing, setEditing] = useState(false)
  const [replying, setReplying] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [replies, setReplies] = useState<Comment[]>([])
  const [repliesOpen, setRepliesOpen] = useState(false)
  const [repliesPage, setRepliesPage] = useState(0)
  const [repliesTotal, setRepliesTotal] = useState(comment.reply_count)
  const [loadingReplies, setLoadingReplies] = useState(false)

  const isMine = user?.id === comment.user_id
  const author = isMine ? user.name : `User #${comment.user_id}`
  const isReply = comment.parent_id !== null

  async function loadReplies(page: number) {
    setLoadingReplies(true)
    setError(null)

    try {
      const envelope = await listReplies(comment.id, { page, per_page: REPLIES_PER_PAGE })
      const batch = envelope.data ?? []

      setReplies((current) => {
        const seen = new Set(current.map((r) => r.id))
        return page === 1 ? batch : [...current, ...batch.filter((r) => !seen.has(r.id))]
      })
      setRepliesTotal(envelope.meta?.total ?? batch.length)
      setRepliesPage(envelope.meta?.page ?? page)
      setRepliesOpen(true)
    } catch (err) {
      setError(toFormError(err).message)
    } finally {
      setLoadingReplies(false)
    }
  }

  async function submitReply(body: string) {
    const created = await replyToComment(comment.id, body)

    // The server may attach it to this comment's own parent when this is
    // already a reply, so only show it here when it truly belongs here.
    if (created.parent_id === comment.id) {
      setReplies((current) => [...current, created])
      setRepliesTotal((n) => n + 1)
      setRepliesOpen(true)
    }

    setReplying(false)
    onReplyAdded()
  }

  async function saveEdit(body: string) {
    onChanged(await updateComment(comment.id, body))
    setEditing(false)
  }

  async function remove() {
    setDeleting(true)
    setError(null)

    try {
      await deleteComment(comment.id)
      setConfirming(false)
      onDeleted(comment.id)
    } catch (err) {
      setError(toFormError(err).message)
      setConfirming(false)
    } finally {
      setDeleting(false)
    }
  }

  const hiddenReplies = repliesTotal - replies.length

  return (
    <article className="flex gap-2">
      <Avatar name={author} size={isReply ? 28 : 34} />

      <div className="min-w-0 flex-1">
        {error && (
          <div className="mb-2">
            <Alert kind="error">{error}</Alert>
          </div>
        )}

        {editing ? (
          <CommentComposer
            placeholder="Edit your comment"
            submitLabel="Save"
            autoFocus
            compact={isReply}
            onSubmit={saveEdit}
            onCancel={() => setEditing(false)}
          />
        ) : (
          <>
            {/* The bubble carries the name and the text; everything actionable
                sits outside it, which is what keeps a dense thread readable. */}
            <div className="inline-block max-w-full rounded-2xl bg-slate-100 px-3 py-2 dark:bg-slate-800">
              <p className="text-[13px] font-semibold">
                {author}
                {isMine && (
                  <span className="ml-1.5 text-xs font-normal text-slate-500 dark:text-slate-400">
                    you
                  </span>
                )}
              </p>
              <p className="text-[15px] leading-snug whitespace-pre-wrap">{comment.body}</p>
            </div>

            <div className="mt-0.5 flex flex-wrap items-center gap-3 px-2 text-xs text-slate-500 dark:text-slate-400">
              <time dateTime={comment.created_at} title={new Date(comment.created_at).toLocaleString()}>
                {relativeTime(comment.created_at)}
              </time>
              {comment.edited && <span>edited</span>}

              <button type="button" onClick={() => setReplying((v) => !v)} className="font-semibold hover:underline">
                Reply
              </button>

              {isMine && (
                <>
                  <button type="button" onClick={() => setEditing(true)} className="font-semibold hover:underline">
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => setConfirming(true)}
                    className="font-semibold text-red-600 hover:underline dark:text-red-400"
                  >
                    Delete
                  </button>
                </>
              )}
            </div>
          </>
        )}

        {replying && (
          <div className="mt-2">
            <CommentComposer
              placeholder={`Reply to ${author}`}
              submitLabel="Reply"
              autoFocus
              compact
              onSubmit={submitReply}
              onCancel={() => setReplying(false)}
            />
          </div>
        )}

        {/* Replies live under the comment they answer, indented once and no
            further — the thread is two levels deep and the layout says so. */}
        {!isReply && (repliesTotal > 0 || replies.length > 0) && (
          <div className="mt-2 space-y-3 border-l border-slate-200 pl-3 dark:border-slate-800">
            {replies.map((reply) => (
              <CommentItem
                key={reply.id}
                comment={reply}
                onChanged={(updated) =>
                  setReplies((current) => current.map((r) => (r.id === updated.id ? updated : r)))
                }
                onDeleted={(id) => {
                  setReplies((current) => current.filter((r) => r.id !== id))
                  setRepliesTotal((n) => Math.max(n - 1, 0))
                  onReplyAdded()
                }}
                onReplyAdded={onReplyAdded}
              />
            ))}

            {hiddenReplies > 0 && (
              <button
                type="button"
                disabled={loadingReplies}
                onClick={() => void loadReplies(repliesOpen ? repliesPage + 1 : 1)}
                className="text-xs font-semibold text-slate-600 hover:underline disabled:opacity-60 dark:text-slate-400"
              >
                {loadingReplies
                  ? 'Loading…'
                  : `View ${hiddenReplies} ${hiddenReplies === 1 ? 'reply' : 'replies'}`}
              </button>
            )}
          </div>
        )}
      </div>

      <ConfirmDialog
        open={confirming}
        busy={deleting}
        title={isReply ? 'Delete this reply?' : 'Delete this comment?'}
        body={
          isReply
            ? 'This cannot be undone.'
            : 'Its replies will be removed with it. This cannot be undone.'
        }
        onConfirm={() => void remove()}
        onCancel={() => setConfirming(false)}
      />
    </article>
  )
}
