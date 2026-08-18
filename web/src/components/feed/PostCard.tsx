import { Link } from 'react-router-dom'

import { useAuth } from '../../auth/context'
import { relativeTime } from '../../lib/time'
import type { Post } from '../../types/api'
import { Avatar } from '../Avatar'
import { CommentIcon, DotsIcon, GlobeIcon, LikeIcon, ShareIcon } from '../icons'

export function PostCard({ post }: { post: Post }) {
  const { user } = useAuth()

  // The API returns user_id but no author record, so a name is only knowable
  // for the signed-in reader's own posts. Everyone else is labelled by id
  // until the backend embeds the author in PostResource.
  const isMine = user?.id === post.user_id
  const author = isMine ? user.name : `User #${post.user_id}`

  return (
    <article className="rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <header className="flex items-start gap-3 p-3 pb-2">
        <Avatar name={author} size={40} />

        <div className="min-w-0 flex-1">
          <p className="truncate text-[15px] font-semibold">
            {author}
            {isMine && (
              <span className="ml-2 rounded bg-slate-100 px-1.5 py-0.5 text-xs font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                you
              </span>
            )}
          </p>
          <p className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            <Link to={`/posts/${post.id}`} className="hover:underline">
              <time dateTime={post.created_at} title={new Date(post.created_at).toLocaleString()}>
                {relativeTime(post.created_at)}
              </time>
            </Link>
            {post.updated_at !== post.created_at && (
              <>
                <span aria-hidden="true">·</span>
                <span>edited</span>
              </>
            )}
            <span aria-hidden="true">·</span>
            <GlobeIcon size={13} />
          </p>
        </div>

        <button
          type="button"
          aria-label="Post options"
          className="rounded-full p-1.5 text-slate-500 transition hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <DotsIcon size={20} />
        </button>
      </header>

      <div className="px-3 pb-3">
        <h2 className="mb-1 text-[17px] font-semibold">
          {/* The title is the link to the post's own page. */}
          <Link to={`/posts/${post.id}`} className="hover:underline">
            {post.title}
          </Link>
        </h2>
        {/* whitespace-pre-wrap so the line breaks the author typed survive */}
        <p className="text-[15px] leading-relaxed whitespace-pre-wrap">{post.content}</p>

        <Link
          to={`/posts/${post.id}`}
          className="mt-2 inline-block text-sm font-medium text-blue-600 hover:underline dark:text-blue-400"
        >
          View post
        </Link>
      </div>

      <footer className="flex border-t border-slate-200 p-1 dark:border-slate-800">
        <Action icon={<LikeIcon />} label="Like" />
        <Link
          to={`/posts/${post.id}`}
          className="flex flex-1 items-center justify-center gap-2 rounded-lg py-2 text-sm
            font-medium text-slate-600 transition hover:bg-slate-100
            dark:text-slate-400 dark:hover:bg-slate-800"
        >
          <CommentIcon />
          Comment
        </Link>
        <Action icon={<ShareIcon />} label="Share" />
      </footer>
    </article>
  )
}

function Action({ icon, label }: { icon: React.ReactNode; label: string }) {
  return (
    <button
      type="button"
      className="flex flex-1 items-center justify-center gap-2 rounded-lg py-2 text-sm
        font-medium text-slate-600 transition hover:bg-slate-100
        dark:text-slate-400 dark:hover:bg-slate-800"
    >
      {icon}
      {label}
    </button>
  )
}

/** Placeholder shown while the first page loads. */
export function PostSkeleton() {
  return (
    <div className="animate-pulse rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900">
      <div className="flex items-center gap-3">
        <div className="h-10 w-10 rounded-full bg-slate-200 dark:bg-slate-800" />
        <div className="flex-1 space-y-2">
          <div className="h-3 w-32 rounded bg-slate-200 dark:bg-slate-800" />
          <div className="h-2.5 w-20 rounded bg-slate-200 dark:bg-slate-800" />
        </div>
      </div>
      <div className="mt-4 space-y-2">
        <div className="h-3.5 w-3/5 rounded bg-slate-200 dark:bg-slate-800" />
        <div className="h-3 w-full rounded bg-slate-200 dark:bg-slate-800" />
        <div className="h-3 w-4/5 rounded bg-slate-200 dark:bg-slate-800" />
      </div>
    </div>
  )
}
