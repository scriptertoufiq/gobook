/**
 * Reactions — frontend only.
 *
 * There is no reactions endpoint, so a choice is stored in this browser and
 * nowhere else: it survives a reload, but nobody else can see it and it does
 * not follow you to another device. That is a deliberate limit of doing this
 * without a backend, not an oversight — when an API exists, `reactionStore`
 * below is the one module that has to change.
 */

export type ReactionKey = 'like' | 'love' | 'care' | 'sad' | 'angry'

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
