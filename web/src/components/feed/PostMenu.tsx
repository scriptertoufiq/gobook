import { useEffect, useRef, useState } from 'react'
import { DotsIcon } from '../icons'

/**
 * The per-post `···` menu.
 *
 * Rendered only for posts the reader may act on — the API allows edits and
 * deletes by the author alone, so offering them to anyone else would just
 * produce a 403. The server enforces it regardless; hiding it is courtesy,
 * not security.
 */
export function PostMenu({ onEdit, onDelete }: { onEdit: () => void; onDelete: () => void }) {
  const [open, setOpen] = useState(false)
  const wrapper = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return

    const onPointerDown = (e: MouseEvent) => {
      if (!wrapper.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }

    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div ref={wrapper} className="relative">
      <button
        type="button"
        aria-label="Post options"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="rounded-full p-1.5 text-slate-500 transition hover:bg-slate-100 dark:hover:bg-slate-800"
      >
        <DotsIcon size={20} />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 z-20 mt-1 w-40 overflow-hidden rounded-lg border
            border-slate-200 bg-white py-1 shadow-lg dark:border-slate-700 dark:bg-slate-800"
        >
          <Item
            onClick={() => {
              setOpen(false)
              onEdit()
            }}
          >
            Edit post
          </Item>
          <Item
            danger
            onClick={() => {
              setOpen(false)
              onDelete()
            }}
          >
            Delete post
          </Item>
        </div>
      )}
    </div>
  )
}

function Item({
  children,
  onClick,
  danger,
}: {
  children: React.ReactNode
  onClick: () => void
  danger?: boolean
}) {
  return (
    <button
      type="button"
      role="menuitem"
      onClick={onClick}
      className={`block w-full px-3 py-2 text-left text-sm font-medium transition
        hover:bg-slate-100 dark:hover:bg-slate-700
        ${danger ? 'text-red-600 dark:text-red-400' : ''}`}
    >
      {children}
    </button>
  )
}
