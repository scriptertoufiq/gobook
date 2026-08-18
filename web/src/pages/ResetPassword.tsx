import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import * as authApi from '../api/auth'
import { Alert, Button, Card, Field } from '../components/ui'
import { toFormError, type FormError } from '../lib/errors'

/**
 * The target of the emailed reset link.
 *
 * The backend cannot complete this flow itself — the user has to type a new
 * password — so AUTH_PASSWORD_RESET_URL points here and the token rides in the
 * query string. Set it to this route's URL (e.g. http://localhost:5173/reset-password),
 * or the link will land on the API instead of the frontend.
 */
export function ResetPassword() {
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''

  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [done, setDone] = useState<string | null>(null)
  const [error, setError] = useState<FormError | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()

    if (password !== confirm) {
      setError({ message: 'Those passwords do not match.', fields: {} })
      return
    }

    setBusy(true)
    setError(null)

    try {
      setDone(await authApi.resetPassword(token, password))
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <h1 className="mb-6 text-xl font-semibold">Choose a new password</h1>

        {!token && (
          <Alert kind="error">
            This link is missing its token. Request a new reset email.
          </Alert>
        )}

        {done ? (
          <div className="space-y-4">
            <Alert kind="success">{done}</Alert>
            <Link to="/login">
              <Button className="w-full">Sign in</Button>
            </Link>
          </div>
        ) : (
          token && (
            <form onSubmit={submit} className="space-y-4">
              {error && <Alert kind="error">{error.message}</Alert>}

              <Field
                label="New password"
                type="password"
                value={password}
                required
                minLength={8}
                autoComplete="new-password"
                error={error?.fields.password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <Field
                label="Confirm new password"
                type="password"
                value={confirm}
                required
                minLength={8}
                autoComplete="new-password"
                onChange={(e) => setConfirm(e.target.value)}
              />

              <Button type="submit" disabled={busy} className="w-full">
                {busy ? 'Updating…' : 'Update password'}
              </Button>
            </form>
          )
        )}

        <p className="mt-5 text-sm">
          <Link to="/login" className="text-blue-600 hover:underline dark:text-blue-400">
            Back to sign in
          </Link>
        </p>
      </Card>
    </div>
  )
}
