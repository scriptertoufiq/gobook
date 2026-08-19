import { ApiError } from '../api/client'
import { removeReaction, setReaction } from '../api/reactions'
import type { ReactionKey } from '../types/api'

/**
 * The offline outbox for reactions.
 *
 * Reactions go to the server. This exists only for the case where they cannot:
 * the request fails to reach the network at all, so the click is parked here
 * and replayed once the connection comes back. It is a delivery buffer, not a
 * store — nothing reads its contents to render anything.
 *
 * The distinction that matters is between a request that never arrived and one
 * that arrived and was refused. A 403 or a 404 will fail identically forever,
 * so queueing it would mean retrying something that can never succeed; only a
 * genuine transport failure is worth keeping.
 */
const STORAGE_KEY = 'reactions.outbox.v1'

/** A reaction waiting to be delivered. A null type means "remove mine". */
export interface QueuedReaction {
  postID: number
  type: ReactionKey | null
  /** When the person actually clicked, so the server can reject a stale replay. */
  actedAt: string
}

type Outbox = Record<string, QueuedReaction>

type Listener = () => void
const listeners = new Set<Listener>()

/** Notified whenever the queue changes, so the UI can show what is pending. */
export function subscribe(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

function notify() {
  listeners.forEach((fn) => fn())
}

function read(): Outbox {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}

    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return {}

    return parsed as Outbox
  } catch {
    // Corrupt or unreadable. Starting clean loses at most a few queued clicks,
    // which beats every post card throwing on render.
    return {}
  }
}

function write(outbox: Outbox) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(outbox))
  } catch {
    // Private browsing, or the quota is full. The reaction is lost rather than
    // delivered late — worth nobody's crash.
  }
  notify()
}

/**
 * Parks a reaction for later delivery.
 *
 * Keyed by post, so clicking repeatedly while offline leaves one entry holding
 * the final intent — the same collapse the server's own pending set does.
 */
export function enqueue(postID: number, type: ReactionKey | null): void {
  const outbox = read()
  outbox[String(postID)] = { postID, type, actedAt: new Date().toISOString() }
  write(outbox)
}

export function dequeue(postID: number): void {
  const outbox = read()
  delete outbox[String(postID)]
  write(outbox)
}

export function queued(postID: number): QueuedReaction | null {
  return read()[String(postID)] ?? null
}

export function pending(): QueuedReaction[] {
  return Object.values(read())
}

export function pendingCount(): number {
  return Object.keys(read()).length
}

/**
 * True when a failure means the request never reached the server.
 *
 * ApiError only exists once a response came back, so anything else — a thrown
 * TypeError from fetch, a DNS failure, an aborted connection — is a transport
 * problem worth retrying. A 5xx is included: the server is there but not
 * answering properly, and the same click will likely work in a minute.
 */
export function isTransportFailure(err: unknown): boolean {
  if (err instanceof ApiError) {
    return err.status >= 500 || err.status === 0
  }
  return true
}

let draining = false

/**
 * Delivers everything waiting.
 *
 * Guarded against overlapping runs: the online event, the interval and a
 * successful click can all trigger a drain at once, and replaying the same
 * entry twice would be wasted requests.
 *
 * Returns the posts it settled, so a caller can refresh exactly those rather
 * than re-reading the whole feed.
 */
export async function drain(): Promise<number[]> {
  if (draining) return []

  const entries = pending()
  if (entries.length === 0) return []

  draining = true
  const delivered: number[] = []

  try {
    for (const entry of entries) {
      try {
        const result =
          entry.type === null
            ? await removeReaction(entry.postID, entry.actedAt)
            : await setReaction(entry.postID, entry.type, entry.actedAt)

        // `applied: false` means the server had something newer. The entry is
        // settled either way — there is nothing left to deliver.
        dequeue(entry.postID)
        delivered.push(entry.postID)

        if (!result.applied) {
          console.info(
            `reactions: queued reaction for post ${entry.postID} was superseded by a newer one`,
          )
        }
      } catch (err) {
        if (isTransportFailure(err)) {
          // Still offline. Leave this and everything after it for the next try.
          break
        }

        // Refused rather than undelivered — a deleted post, or a rejected
        // value. Retrying cannot help, so drop it.
        dequeue(entry.postID)
        delivered.push(entry.postID)
      }
    }
  } finally {
    draining = false
  }

  return delivered
}

/**
 * Starts draining whenever the connection returns.
 *
 * Both signals are used because neither is sufficient: `online` does not fire
 * for a connection that is up but useless — a captive portal, a dead uplink —
 * and a bare interval would take up to its full period to notice a laptop
 * waking up.
 */
export function watchConnection(onDrained: (postIDs: number[]) => void): () => void {
  const attempt = () => {
    void drain().then((delivered) => {
      if (delivered.length > 0) onDrained(delivered)
    })
  }

  window.addEventListener('online', attempt)
  const timer = setInterval(attempt, 30_000)
  attempt() // and once now, for whatever an earlier session left behind

  return () => {
    window.removeEventListener('online', attempt)
    clearInterval(timer)
  }
}
