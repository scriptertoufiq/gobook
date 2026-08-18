import type { FeedPost } from '../../data/feed'
import { Avatar } from '../Avatar'
import { CloseIcon, CommentIcon, DotsIcon, GlobeIcon, LikeIcon, ShareIcon } from '../icons'

export function PostCard({ post }: { post: FeedPost }) {
  return (
    <article className="rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
      {/* Header */}
      <header className="flex items-start gap-3 p-3 pb-2">
        <Avatar name={post.context ?? post.author} size={40} />

        <div className="min-w-0 flex-1">
          <p className="truncate text-[15px] font-semibold">{post.context ?? post.author}</p>
          <p className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            {post.context && <span className="truncate">{post.author}</span>}
            {post.context && <span aria-hidden="true">·</span>}
            <span>{post.postedAt}</span>
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
        <button
          type="button"
          aria-label="Hide post"
          className="rounded-full p-1.5 text-slate-500 transition hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <CloseIcon size={20} />
        </button>
      </header>

      {/* Body */}
      <p className="px-3 pb-3 text-[15px] leading-relaxed">{post.body}</p>

      {/* Media — a gradient panel standing in for the eventual image */}
      {post.media && (
        <div
          className={`flex aspect-[16/11] flex-col items-center justify-center gap-2
            bg-gradient-to-b px-8 text-center ${post.media}`}
        >
          <h3 className="text-2xl font-bold text-white drop-shadow-sm sm:text-3xl">
            {post.mediaTitle}
          </h3>
          {post.mediaSubtitle && (
            <p className="text-sm text-white/90 sm:text-base">{post.mediaSubtitle}</p>
          )}
        </div>
      )}

      {/* Counts */}
      <div className="flex items-center justify-between px-3 py-2 text-xs text-slate-500 dark:text-slate-400">
        <span className="flex items-center gap-1.5">
          <span className="flex h-4.5 w-4.5 items-center justify-center rounded-full bg-blue-600 text-white">
            <LikeIcon size={11} />
          </span>
          {post.likes.toLocaleString()}
        </span>
        <span>
          {post.comments} comments · {post.shares} shares
        </span>
      </div>

      {/* Actions */}
      <footer className="flex border-t border-slate-200 p-1 dark:border-slate-800">
        <Action icon={<LikeIcon />} label="Like" />
        <Action icon={<CommentIcon />} label="Comment" />
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
