import { useAuth } from '../auth/context'
import { Composer } from '../components/feed/Composer'
import { LeftRail } from '../components/feed/LeftRail'
import { PostCard } from '../components/feed/PostCard'
import { RightRail } from '../components/feed/RightRail'
import { PencilIcon } from '../components/icons'
import { posts } from '../data/feed'

/**
 * The landing page after login.
 *
 * Content is static — see src/data/feed.ts, the single module to replace when
 * this goes live. The layout is three independently scrolling columns: the
 * rails are `sticky` under the header so only the feed moves, which is what
 * keeps navigation reachable on a long timeline.
 */
export function Home() {
  const { user } = useAuth()
  const name = user?.name ?? 'there'

  return (
    <div className="mx-auto flex max-w-[1600px] justify-center gap-4 px-2 py-4">
      {/* Left rail */}
      <div className="sticky top-[4.5rem] hidden h-[calc(100vh-5.5rem)] w-72 shrink-0 overflow-y-auto pr-1 lg:block">
        <LeftRail name={name} />
      </div>

      {/* Feed */}
      <main className="w-full max-w-xl min-w-0 space-y-3">
        <Composer name={name} />
        {posts.map((post) => (
          <PostCard key={post.id} post={post} />
        ))}

        <p className="py-6 text-center text-sm text-slate-500 dark:text-slate-500">
          You&rsquo;re all caught up.
        </p>
      </main>

      {/* Right rail */}
      <div className="sticky top-[4.5rem] hidden h-[calc(100vh-5.5rem)] w-80 shrink-0 overflow-y-auto pl-1 xl:block">
        <RightRail />
      </div>

      {/* Compose shortcut, mirroring the floating action button in the reference */}
      <button
        type="button"
        aria-label="Create post"
        className="fixed right-5 bottom-5 z-20 flex h-12 w-12 items-center justify-center
          rounded-full bg-slate-200 text-slate-800 shadow-lg transition hover:bg-slate-300
          dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700"
      >
        <PencilIcon size={22} />
      </button>
    </div>
  )
}
