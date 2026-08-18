import { useCallback, useEffect, useRef, useState } from 'react'
import { listPosts } from '../../api/posts'
import { toFormError } from '../../lib/errors'
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
  const [lastPage, setLastPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const sentinel = useRef<HTMLDivElement>(null)
  // Guards against the observer firing again while a request is already out —
  // state updates are async, so `loading` alone would let two fire.
  const inFlight = useRef(false)

  const hasMore = page < lastPage

  const load = useCallback(async (target: number) => {
    if (inFlight.current) return
    inFlight.current = true
    setLoading(true)
    setError(null)

    try {
      const envelope = await listPosts({
        page: target,
        per_page: PER_PAGE,
        sort_by: 'created_at',
        sort_dir: 'desc',
      })

      const batch = envelope.data ?? []

      setPosts((current) => {
        // Offset pagination shifts when a post is created mid-scroll, which can
        // hand back a row already on screen. Keyed by id, so a repeat is
        // dropped rather than rendered twice.
        const seen = new Set(current.map((p) => p.id))
        return [...current, ...batch.filter((p) => !seen.has(p.id))]
      })

      setPage(envelope.meta?.page ?? target)
      setLastPage(envelope.meta?.last_page ?? target)
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
  useEffect(() => {
    const node = sentinel.current
    if (!node || !hasMore || error) return

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
  }, [hasMore, page, load, error])

  function prepend(post: Post) {
    setPosts((current) => [post, ...current])
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
        <PostCard key={post.id} post={post} />
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
