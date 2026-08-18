import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import * as authApi from '../api/auth'
import { AuthLayout } from '../components/AuthLayout'
import { Alert, PillButton, PillField } from '../components/ui'
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
  const navigate = useNavigate()
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
    <AuthLayout
      title="Choose a new password"
      back="/login"
      footer={
        <PillButton variant="outline" type="button" onClick={() => navigate('/login')}>
          Back to login
        </PillButton>
      }
    >
      {!token && <Alert kind="error">This link is missing its token. Request a new reset email.</Alert>}

      {done && <Alert kind="success">{done}</Alert>}

      {token && !done && (
        <form onSubmit={submit} className="space-y-3">
          {error && <Alert kind="error">{error.message}</Alert>}

          <PillField
            type="password"
            placeholder="New password"
            value={password}
            required
            minLength={8}
            autoComplete="new-password"
            error={error?.fields.password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <PillField
            type="password"
            placeholder="Confirm new password"
            value={confirm}
            required
            minLength={8}
            autoComplete="new-password"
            onChange={(e) => setConfirm(e.target.value)}
          />

          <PillButton type="submit" disabled={busy} className="!mt-4">
            {busy ? 'Updating…' : 'Update password'}
          </PillButton>
        </form>
      )}
    </AuthLayout>
  )
}
