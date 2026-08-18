import type { ReactionKey } from './reactions'

/**
 * Per-browser storage for reactions.
 *
 * Versioned key so a future shape change can be ignored rather than crash on
 * whatever is already in someone's localStorage. Reads are defensive for the
 * same reason: this data is outside the app's control and a user can edit it.
 */
const STORAGE_KEY = 'reactions.v1'

type Store = Record<string, ReactionKey>

type Listener = () => void
const listeners = new Set<Listener>()

/** Subscribe to changes so every card showing a post stays in step. */
export function subscribe(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

function read(): Store {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}

    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return {}

    return parsed as Store
  } catch {
    // Corrupt or unreadable: start clean rather than break every post card.
    return {}
  }
}

function write(store: Store): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(store))
  } catch {
    // Private browsing, or the quota is full. A lost reaction is not worth
    // failing the render over.
  }
  listeners.forEach((fn) => fn())
}

export function getReaction(postID: number): ReactionKey | null {
  return read()[String(postID)] ?? null
}

/**
 * Sets a reaction, or clears it when the same one is chosen again — which is
 * how every reaction UI behaves, and the only way to undo without a separate
 * control.
 */
export function toggleReaction(postID: number, key: ReactionKey): ReactionKey | null {
  const store = read()
  const id = String(postID)

  if (store[id] === key) {
    delete store[id]
    write(store)
    return null
  }

  store[id] = key
  write(store)
  return key
}

export function clearReaction(postID: number): void {
  const store = read()
  delete store[String(postID)]
  write(store)
}
