/**
 * How each reaction is presented. The keys themselves come from types/api,
 * where they mirror the set the server accepts.
 */

import type { ReactionKey } from '../types/api'

export type { ReactionKey }

export interface Reaction {
  key: ReactionKey
  label: string
  emoji: string
  /** Colour for the label once this reaction is the active one. */
  tone: string
  /** Background for the little circular chip in the summary row. */
  chip: string
}

// Order is the order they appear in the picker.
export const reactions: Reaction[] = [
  { key: 'like', label: 'Like', emoji: '👍', tone: 'text-blue-600 dark:text-blue-400', chip: 'bg-blue-600' },
  { key: 'love', label: 'Love', emoji: '❤️', tone: 'text-rose-600 dark:text-rose-400', chip: 'bg-rose-600' },
  { key: 'care', label: 'Care', emoji: '🤗', tone: 'text-amber-600 dark:text-amber-400', chip: 'bg-amber-500' },
  { key: 'sad', label: 'Sad', emoji: '😢', tone: 'text-amber-600 dark:text-amber-400', chip: 'bg-amber-500' },
  { key: 'angry', label: 'Angry', emoji: '😡', tone: 'text-orange-700 dark:text-orange-500', chip: 'bg-orange-600' },
]

const byKey = new Map(reactions.map((r) => [r.key, r]))

export function reactionFor(key: ReactionKey | null): Reaction | null {
  return key ? (byKey.get(key) ?? null) : null
}
