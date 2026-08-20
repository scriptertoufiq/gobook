import { useCallback, useEffect, useState } from 'react'
import { deleteUser, listUsers } from '../api/users'
import { useAuth } from '../auth/context'
import { Alert, Badge, Button, Card } from '../components/ui'
import { toFormError } from '../lib/errors'
import type { PaginationMeta, User } from '../types/api'

/** Admin-only: the listing exposes every account's email address. */
export function Users() {
  const { user: current } = useAuth()

  const [users, setUsers] = useState<User[]>([])
  const [meta, setMeta] = useState<PaginationMeta | null>(null)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const envelope = await listUsers({ page, search, per_page: 15 })
      setUsers(envelope.data ?? [])
      setMeta(envelope.meta ?? null)
    } catch (err) {
      setError(toFormError(err).message)
    } finally {
      setLoading(false)
    }
  }, [page, search])

  // Debounced so typing in the search box doesn't fire a request per keystroke.
  useEffect(() => {
    const timer = setTimeout(() => void load(), 250)
    return () => clearTimeout(timer)
  }, [load])

  async function remove(target: User) {
    if (!confirm(`Delete ${target.name} (${target.email})?`)) return
    try {
      await deleteUser(target.id)
      await load()
    } catch (err) {
      setError(toFormError(err).message)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Users</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {meta?.total !== undefined ? `${meta.total} account${meta.total === 1 ? '' : 's'}` : '—'}
          </p>
        </div>
        <input
          value={search}
          placeholder="Search name or email…"
          onChange={(e) => {
            setPage(1)
            setSearch(e.target.value)
          }}
          className="rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none
            focus:border-blue-500 focus:ring-2 focus:ring-blue-500/40
            dark:border-slate-700 dark:bg-slate-900"
        />
      </div>

      {error && <Alert kind="error">{error}</Alert>}

      <Card className="overflow-x-auto p-0">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-slate-200 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
            <tr>
              <th className="px-4 py-3 font-medium">Name</th>
              <th className="px-4 py-3 font-medium">Email</th>
              <th className="px-4 py-3 font-medium">Role</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-slate-500">
                  Loading…
                </td>
              </tr>
            )}

            {!loading && users.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-slate-500">
                  No users match that search.
                </td>
              </tr>
            )}

            {!loading &&
              users.map((u) => (
                <tr key={u.id} className="border-b border-slate-100 last:border-0 dark:border-slate-800">
                  <td className="px-4 py-3 font-medium">{u.name}</td>
                  <td className="px-4 py-3 text-slate-600 dark:text-slate-400">{u.email}</td>
                  <td className="px-4 py-3">
                    <Badge tone={u.role === 'admin' ? 'amber' : 'slate'}>{u.role}</Badge>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1.5">
                      <Badge tone={u.is_active ? 'green' : 'slate'}>
                        {u.is_active ? 'active' : 'disabled'}
                      </Badge>
                      {!u.email_verified && <Badge tone="amber">unverified</Badge>}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-right">
                    {/* The API refuses this anyway; hiding it avoids a pointless 403. */}
                    {u.id !== current?.id && (
                      <Button variant="ghost" onClick={() => void remove(u)} className="!px-2 !py-1 text-xs">
                        Delete
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </Card>

      {meta && (meta.last_page ?? 1) > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-slate-500 dark:text-slate-400">
            Page {meta.page} of {meta.last_page ?? 1}
          </span>
          <div className="flex gap-2">
            <Button variant="ghost" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              Previous
            </Button>
            <Button
              variant="ghost"
              disabled={page >= (meta.last_page ?? 1)}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
