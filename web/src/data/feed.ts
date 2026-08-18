/**
 * Static placeholder content for the home feed.
 *
 * Everything the feed renders comes from this one module, so wiring it to the
 * API later is a matter of replacing these exports with fetches — the
 * components never see where the data came from. The shapes below are what the
 * eventual `/api/v1/posts` resource should return.
 */

export interface FeedPost {
  id: number
  author: string
  /** Group, page or context the post was published in. */
  context?: string
  postedAt: string
  body: string
  /** Tailwind gradient classes standing in for an attached image. */
  media?: string
  mediaTitle?: string
  mediaSubtitle?: string
  likes: number
  comments: number
  shares: number
}

export const posts: FeedPost[] = [
  {
    id: 1,
    author: 'M Asif Rahman',
    context: 'GoBook Self Hosting Community',
    postedAt: '16h',
    body: 'Managed Harness Hosting is here: keeps agents working in real-world environments. Hey everyone — feeling excited to finally ship this one.',
    media: 'from-sky-200 to-blue-600',
    mediaTitle: 'Managed Harness Hosting Is Here',
    mediaSubtitle: 'keeps agents working in real-world environments',
    likes: 248,
    comments: 31,
    shares: 12,
  },
  {
    id: 2,
    author: 'Nahid Hasan',
    postedAt: '4h',
    body: 'Shipped the new migration runner today. One self-registering file per change means no central list to conflict over — a small thing that removes a whole category of merge pain.',
    likes: 96,
    comments: 14,
    shares: 3,
  },
  {
    id: 3,
    author: 'Jane Editor',
    context: 'Go Developers BD',
    postedAt: '9h',
    body: 'Reminder that rotating refresh tokens only actually rotate if the revoking UPDATE is the critical section. Checking "is it usable" first and revoking after is a race, not a guard.',
    media: 'from-violet-300 to-indigo-700',
    mediaTitle: 'Token rotation, done right',
    mediaSubtitle: 'one winner per exchange, always',
    likes: 412,
    comments: 57,
    shares: 40,
  },
]

export interface Shortcut {
  id: number
  name: string
}

export const shortcuts: Shortcut[] = [
  { id: 1, name: 'Haier Ac' },
  { id: 2, name: 'Nahids Blogs' },
  { id: 3, name: 'Tester Nahid' },
  { id: 4, name: 'Testoriam' },
]

export interface Sponsored {
  id: number
  title: string
  domain: string
  media: string
}

export const sponsored: Sponsored[] = [
  { id: 1, title: 'Flex your journey', domain: 'airlines.example.com', media: 'from-slate-300 to-slate-600' },
  { id: 2, title: 'Earn more on weekends', domain: 'partner.example.com', media: 'from-pink-300 to-rose-600' },
]

export interface FriendRequest {
  id: number
  name: string
  mutual: number
  age: string
}

export const friendRequests: FriendRequest[] = [
  { id: 1, name: 'Md Zehad Mollick', mutual: 746, age: '3d' },
]

export interface Contact {
  id: number
  name: string
  online: boolean
}

export const contacts: Contact[] = [
  { id: 1, name: 'Asif Rahman', online: true },
  { id: 2, name: 'Jane Editor', online: true },
  { id: 3, name: 'Nahid Hasan', online: false },
  { id: 4, name: 'Zehad Mollick', online: true },
  { id: 5, name: 'Tester Nahid', online: false },
]
