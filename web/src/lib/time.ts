/**
 * Compact relative time, the way a feed shows it: "just now", "4h", "3d".
 * Falls back to a date once something is older than a week, since "412d" tells
 * a reader less than the date does.
 */
export function relativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''

  const seconds = Math.floor((Date.now() - then) / 1000)

  if (seconds < 45) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d`

  return new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
}
