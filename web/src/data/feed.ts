/**
 * Static placeholder content for the home page's side rails.
 *
 * The post list is no longer here — the feed reads /api/v1/posts through
 * src/api/posts.ts. What remains is the decoration around it: shortcuts,
 * sponsored slots, friend requests and contacts, none of which have an
 * endpoint behind them yet.
 */

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
