import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { deletePost, getPost, type PostWithSource } from '../api/posts'
import { useAuth } from '../auth/context'
import { Avatar } from '../components/Avatar'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { PostEditor } from '../components/feed/PostEditor'
import { ReactionButton, ReactionSummary } from '../components/feed/ReactionBar'
import { GlobeIcon } from '../components/icons'
import { Alert, Badge, Button, Card } from '../components/ui'
import { ApiError } from '../api/client'
import { toFormError } from '../lib/errors'
import { relativeTime } from '../lib/time'

/**
 * A post on its own page.
 *
 * The id comes from the URL, so this is reachable by link, bookmark and reload
 * — which is the point of giving a post an address at all. That also means it
 * cannot assume the feed has already loaded: it fetches by id every time.
 */
export function PostDetail() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()

  const [result, setResult] = useState<PostWithSource | null>(null)
  const [error, setError] = useState<{ message: string; status: number } | null>(null)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const postID = Number(id)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)

    if (!Number.isInteger(postID) || postID < 1) {
      setError({ message: 'That is not a valid post id.', status: 400 })
      setLoading(false)
      return
    }

    try {
      setResult(await getPost(postID))
    } catch (err) {
      const status = err instanceof ApiError ? err.status : 0
      const message = err instanceof Error ? err.message : 'Something went wrong.'
      setError({ message, status })
    } finally {
      setLoading(false)
    }
  }, [postID])

  useEffect(() => {
    void load()
  }, [load])

  if (loading) {
    return (
      <Card className="animate-pulse">
        <div className="flex items-center gap-3">
          <div className="h-11 w-11 rounded-full bg-slate-200 dark:bg-slate-800" />
          <div className="flex-1 space-y-2">
            <div className="h-3 w-40 rounded bg-slate-200 dark:bg-slate-800" />
            <div className="h-2.5 w-24 rounded bg-slate-200 dark:bg-slate-800" />
          </div>
        </div>
        <div className="mt-6 space-y-3">
          <div className="h-6 w-3/4 rounded bg-slate-200 dark:bg-slate-800" />
          <div className="h-3 w-full rounded bg-slate-200 dark:bg-slate-800" />
          <div className="h-3 w-5/6 rounded bg-slate-200 dark:bg-slate-800" />
        </div>
      </Card>
    )
  }

  if (error || !result) {
    const missing = error?.status === 404
    return (
      <Card className="text-center">
        <h1 className="text-lg font-semibold">
          {missing ? 'This post no longer exists' : 'Could not load this post'}
        </h1>
        <p className="mx-auto mt-2 max-w-md text-sm text-slate-500 dark:text-slate-400">
          {missing
            ? 'It may have been deleted by its author, or the link may be wrong.'
            : (error?.message ?? 'Something went wrong.')}
        </p>
        <div className="mt-5 flex justify-center gap-2">
          <Button onClick={() => navigate('/')}>Back to feed</Button>
          {!missing && (
            <Button variant="ghost" onClick={() => void load()}>
              Try again
            </Button>
          )}
        </div>
      </Card>
    )
  }

  const { post, servedFrom } = result
  const isMine = user?.id === post.user_id
  const author = isMine ? user.name : `User #${post.user_id}`
  const edited = post.updated_at !== post.created_at

  async function remove() {
    setDeleting(true)
    setActionError(null)

    try {
      await deletePost(post.id)
      // The post no longer exists, so staying here would only render its own
      // 404 — go back to the feed instead.
      navigate('/', { replace: true })
    } catch (err) {
      setActionError(toFormError(err).message)
      setConfirming(false)
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="space-y-3">
      <Link
        to="/"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-slate-600
          hover:underline dark:text-slate-400"
      >
        <svg width="16" height="16" viewBox="0 0 20 20" fill="none" aria-hidden="true">
          <path
            d="M12.5 16 6.5 10l6-6"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        Back to feed
      </Link>

      <Card>
        <header className="flex items-start gap-3">
          <Avatar name={author} size={44} />
          <div className="min-w-0 flex-1">
            <p className="flex items-center gap-2 font-semibold">
              {author}
              {isMine && <Badge tone="slate">you</Badge>}
            </p>
            <p className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
              <time dateTime={post.created_at} title={new Date(post.created_at).toLocaleString()}>
                {relativeTime(post.created_at)}
              </time>
              {edited && (
                <>
                  <span aria-hidden="true">·</span>
                  <span title={new Date(post.updated_at).toLocaleString()}>edited</span>
                </>
              )}
              <span aria-hidden="true">·</span>
              <GlobeIcon size={13} />
            </p>
          </div>
        </header>

        {actionError && (
          <div className="mt-4">
            <Alert kind="error">{actionError}</Alert>
          </div>
        )}

        {editing ? (
          <div className="mt-5">
            <PostEditor
              post={post}
              onCancel={() => setEditing(false)}
              onSaved={(updated) => {
                // The PATCH response carries the saved post but not a cache
                // source, so the note is cleared rather than guessed at. The
                // server wrote this version through on the way out, so a reload
                // will report it as served from cache.
                setResult({ post: updated, servedFrom: null })
                setEditing(false)
              }}
            />
          </div>
        ) : (
          <>
            <h1 className="mt-5 text-2xl font-bold tracking-tight">{post.title}</h1>

            {/* whitespace-pre-wrap so the author's own line breaks survive */}
            <div className="mt-3 text-[15px] leading-relaxed whitespace-pre-wrap">{post.content}</div>

            <div className="mt-6 border-t border-slate-200 pt-3 dark:border-slate-800">
              <div className="mb-1 px-1 text-xs text-slate-500 dark:text-slate-400">
                <ReactionSummary postID={post.id} />
              </div>
              <div className="flex">
                <ReactionButton postID={post.id} />
              </div>
            </div>

            {isMine && (
              <div className="mt-5 flex gap-2">
                <Button variant="ghost" onClick={() => setEditing(true)}>
                  Edit post
                </Button>
                <Button variant="danger" onClick={() => setConfirming(true)}>
                  Delete post
                </Button>
              </div>
            )}
          </>
        )}

        <dl className="mt-6 grid gap-3 border-t border-slate-200 pt-4 text-xs sm:grid-cols-3 dark:border-slate-800">
          <div>
            <dt className="text-slate-500 dark:text-slate-400">Post ID</dt>
            <dd className="mt-0.5 font-medium">#{post.id}</dd>
          </div>
          <div>
            <dt className="text-slate-500 dark:text-slate-400">Published</dt>
            <dd className="mt-0.5 font-medium">{new Date(post.created_at).toLocaleString()}</dd>
          </div>
          <div>
            <dt className="text-slate-500 dark:text-slate-400">Last updated</dt>
            <dd className="mt-0.5 font-medium">{new Date(post.updated_at).toLocaleString()}</dd>
          </div>
        </dl>
      </Card>

      {/* The API tells us whether Redis or MySQL answered. Surfacing it makes
          the cache observable from the page it serves. */}
      <ConfirmDialog
        open={confirming}
        busy={deleting}
        title="Delete this post?"
        body={`"${post.title}" will be removed. This cannot be undone.`}
        onConfirm={() => void remove()}
        onCancel={() => setConfirming(false)}
      />

      {servedFrom && (
        <Alert kind="info">
          <span className="font-medium">{servedFrom}</span>{' '}
          <span className="opacity-75">
            Reload to see it switch — the first read fills the cache, the next is served from it.
          </span>
        </Alert>
      )}
    </div>
  )
}
