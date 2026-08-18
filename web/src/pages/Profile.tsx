import { useState, type FormEvent } from 'react'
import * as authApi from '../api/auth'
import { useAuth } from '../auth/context'
import { Alert, Badge, Button, Card, Field } from '../components/ui'
import { toFormError, type FormError } from '../lib/errors'

export function Profile() {
  const { user, reload, logout } = useAuth()

  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [error, setError] = useState<FormError | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  if (!user) return null

  async function changePassword(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)
    setNotice(null)

    try {
      // This revokes every session. The backend hands back a fresh pair so this
      // device stays signed in; api/auth stores it.
      await authApi.changePassword(current, next)
      setCurrent('')
      setNext('')
      setNotice('Password updated. All other sessions have been signed out.')
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  async function resend() {
    setNotice(null)
    setError(null)
    try {
      setNotice(await authApi.resendVerification())
    } catch (err) {
      setError(toFormError(err))
    }
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Your account</h1>

      <Card>
        <dl className="grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs uppercase tracking-wide text-slate-500">Name</dt>
            <dd className="mt-0.5 font-medium">{user.name}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase tracking-wide text-slate-500">Email</dt>
            <dd className="mt-0.5 font-medium">{user.email}</dd>
          </div>
          <div>
            <dt className="text-xs uppercase tracking-wide text-slate-500">Role</dt>
            <dd className="mt-1">
              <Badge tone={user.role === 'admin' ? 'amber' : 'slate'}>{user.role}</Badge>
            </dd>
          </div>
          <div>
            <dt className="text-xs uppercase tracking-wide text-slate-500">Email verified</dt>
            <dd className="mt-1 flex items-center gap-2">
              <Badge tone={user.email_verified ? 'green' : 'amber'}>
                {user.email_verified ? 'verified' : 'not verified'}
              </Badge>
              {!user.email_verified && (
                <button
                  onClick={() => void resend()}
                  className="text-xs text-blue-600 hover:underline dark:text-blue-400"
                >
                  Resend link
                </button>
              )}
            </dd>
          </div>
        </dl>

        <div className="mt-5 flex gap-2 border-t border-slate-200 pt-4 dark:border-slate-800">
          <Button variant="ghost" onClick={() => void reload()}>
            Refresh
          </Button>
          <Button variant="ghost" onClick={() => void logout(true)}>
            Sign out everywhere
          </Button>
        </div>
      </Card>

      <Card>
        <h2 className="mb-1 font-semibold">Change password</h2>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          Your current password is always required — an access token proves a past login, not
          that you know the credential.
        </p>

        <form onSubmit={changePassword} className="max-w-sm space-y-4">
          {error && <Alert kind="error">{error.message}</Alert>}
          {notice && <Alert kind="success">{notice}</Alert>}

          <Field
            label="Current password"
            type="password"
            value={current}
            required
            autoComplete="current-password"
            error={error?.fields.current_password}
            onChange={(e) => setCurrent(e.target.value)}
          />
          <Field
            label="New password"
            type="password"
            value={next}
            required
            minLength={8}
            autoComplete="new-password"
            error={error?.fields.password}
            onChange={(e) => setNext(e.target.value)}
          />

          <Button type="submit" disabled={busy}>
            {busy ? 'Updating…' : 'Update password'}
          </Button>
        </form>
      </Card>
    </div>
  )
}
