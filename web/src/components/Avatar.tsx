/**
 * Initials avatar.
 *
 * No user-uploaded images exist yet, and a broken <img> looks worse than a
 * deliberate placeholder. The colour is derived from the name so the same
 * person is always the same swatch — recognisable at a glance in a list.
 */
const palette = [
  'bg-blue-600',
  'bg-emerald-600',
  'bg-violet-600',
  'bg-rose-600',
  'bg-amber-600',
  'bg-cyan-600',
  'bg-indigo-600',
  'bg-teal-600',
]

function initials(name: string): string {
  const words = name.trim().split(/\s+/).slice(0, 2)
  return words.map((w) => w[0] ?? '').join('').toUpperCase() || '?'
}

function swatch(name: string): string {
  let sum = 0
  for (let i = 0; i < name.length; i++) sum += name.charCodeAt(i)
  return palette[sum % palette.length]
}

export function Avatar({
  name,
  size = 40,
  ring = false,
}: {
  name: string
  size?: number
  ring?: boolean
}) {
  return (
    <span
      style={{ width: size, height: size, fontSize: Math.round(size * 0.38) }}
      className={`inline-flex shrink-0 items-center justify-center rounded-full font-semibold
        text-white select-none ${swatch(name)} ${ring ? 'ring-2 ring-blue-500 ring-offset-2 dark:ring-offset-slate-900' : ''}`}
      aria-hidden="true"
    >
      {initials(name)}
    </span>
  )
}
