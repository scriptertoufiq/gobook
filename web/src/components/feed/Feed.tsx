import { useCallback, useEffect, useRef, useState } from 'react'
import { getReactions } from '../../api/reactions'
import { listPosts } from '../../api/posts'
import { toFormError } from '../../lib/errors'
import { watchConnection } from '../../lib/reactionStore'
import type { Post } from '../../types/api'
import { Alert, Button } from '../ui'
import { Composer } from './Composer'
import { PostCard, PostSkeleton } from './PostCard'

const PER_PAGE = 10

/**
 * The post list, loaded a page at a time and extended as the reader reaches the
 * bottom.
 *
 * Loading is driven by an IntersectionObserver on a sentinel element rather
 * than a scroll handler: the browser reports the intersection itself, so there
 * is no listener firing on every frame and no threshold arithmetic against
 * scrollHeight.
 */
export function Feed({ name }: { name: string }) {
  const [posts, setPosts] = useState<Post[]>([])
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const sentinel = useRef<HTMLDivElement>(null)
  // Guards against the observer firing again while a request is already out —
  // state updates are async, so `loading` alone would let two fire.
  const inFlight = useRef(false)

  const load = useCallback(async (target: number) => {
    if (inFlight.current) return
    inFlight.current = true
    setLoading(true)
    setError(null)

    try {
      // No sort_by: the default is `id desc`, which is the primary key and
      // therefore free. Asking for created_at instead means sorting the whole
      // table on every page — measured at 4.8 seconds against 0.2 with the
      // default — and for an auto-increment id the two orders are the same
      // thing anyway.
      const envelope = await listPosts({ page: target, per_page: PER_PAGE })

      const batch = envelope.data ?? []

      setPosts((current) => {
        // Offset pagination shifts when a post is created mid-scroll, which can
        // hand back a row already on screen. Keyed by id, so a repeat is
        // dropped rather than rendered twice.
        const seen = new Set(current.map((p) => p.id))
        return [...current, ...batch.filter((p) => !seen.has(p.id))]
      })

      setPage(envelope.meta?.page ?? target)
        // has_more rather than a page count: the feed is not told how many
        // posts exist, only whether to ask again.
        setHasMore(envelope.meta?.has_more ?? false)
    } catch (err) {
      setError(toFormError(err).message)
    } finally {
      inFlight.current = false
      setLoading(false)
      setReady(true)
    }
  }, [])

  // First page.
  useEffect(() => {
    void load(1)
  }, [load])

  // Extend as the sentinel comes into view.
  //
  // A previous failure deliberately does not disarm this. Guarding on `error`
  // would mean one timeout leaves the feed permanently unable to load more
  // until the reader finds the Retry button — scrolling would simply stop.
  useEffect(() => {
    const node = sentinel.current
    if (!node || !hasMore) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) void load(page + 1)
      },
      // Start fetching before the sentinel is actually visible, so the next
      // page is usually there by the time the reader arrives.
      { rootMargin: '400px' },
    )

    observer.observe(node)
    return () => observer.disconnect()
  }, [hasMore, page, load])

  // Anything reacted to while the connection was down is delivered as soon as
  // it returns. Only the posts the queue actually settled are re-read — the
  // server may have resolved a conflict differently from the optimistic guess
  // still on screen.
  useEffect(
    () =>
      watchConnection((postIDs) => {
        postIDs.forEach((id) => {
          void getReactions(id)
            .then((reactions) =>
              setPosts((current) =>
                current.map((p) => (p.id === id ? { ...p, reactions } : p)),
              ),
            )
            .catch(() => {
              // A refresh that fails leaves the optimistic tally in place,
              // which is still the best guess available.
            })
        })
      }),
    [],
  )

  function prepend(post: Post) {
    setPosts((current) => [post, ...current])
  }

  // Edits and deletes are applied to the list in place rather than by
  // refetching: a refetch would reset scroll position and re-request every page
  // the reader has already loaded.
  function replace(updated: Post) {
    setPosts((current) => current.map((p) => (p.id === updated.id ? updated : p)))
  }

  function remove(id: number) {
    setPosts((current) => current.filter((p) => p.id !== id))
  }

  return (
    <div className="space-y-3">
      <Composer name={name} onCreated={prepend} />

      {error && (
        <Alert kind="error">
          <div className="flex items-center justify-between gap-3">
            <span>{error}</span>
            <Button variant="ghost" onClick={() => void load(page)} className="!py-1 !text-xs">
              Retry
            </Button>
          </div>
        </Alert>
      )}

      {!ready && (
        <>
          <PostSkeleton />
          <PostSkeleton />
        </>
      )}

      {posts.map((post) => (
        <PostCard key={post.id} post={post} onUpdated={replace} onDeleted={remove} />
      ))}

      {ready && posts.length === 0 && !error && (
        <div className="rounded-xl border border-slate-200 bg-white p-10 text-center dark:border-slate-800 dark:bg-slate-900">
          <p className="font-medium">No posts yet</p>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Write the first one using the box above.
          </p>
        </div>
      )}

      {/* The sentinel: when this scrolls into range, the next page loads. */}
      <div ref={sentinel} aria-hidden="true" />

      {loading && ready && <PostSkeleton />}

      {ready && !hasMore && posts.length > 0 && (
        <p className="py-6 text-center text-sm text-slate-500 dark:text-slate-500">
          You&rsquo;re all caught up.
        </p>
      )}
    </div>
  )
}
