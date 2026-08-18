import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import * as authApi from '../api/auth'
import { AuthLayout } from '../components/AuthLayout'
import { Alert, PillButton, PillField } from '../components/ui'
import { toFormError } from '../lib/errors'

export function ForgotPassword() {
  const navigate = useNavigate()

  const [email, setEmail] = useState('')
  const [sent, setSent] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)

    try {
      // The backend answers identically whether or not the address exists, so
      // there is nothing here to branch on — and deliberately so: any visible
      // difference would make this an account-enumeration oracle.
      setSent(await authApi.forgotPassword(email))
    } catch (err) {
      setError(toFormError(err).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthLayout
      title="Find your account"
      back="/login"
      footer={
        <PillButton variant="outline" type="button" onClick={() => navigate('/login')}>
          Back to login
        </PillButton>
      }
    >
      {sent ? (
        <Alert kind="success">{sent}</Alert>
      ) : (
        <form onSubmit={submit} className="space-y-3">
          {error && <Alert kind="error">{error}</Alert>}

          <p className="pb-1 text-sm text-slate-600 dark:text-slate-400">
            Enter your email address and we'll send a reset link if it has an account.
          </p>

          <PillField
            type="email"
            placeholder="Email address"
            value={email}
            required
            autoComplete="email"
            onChange={(e) => setEmail(e.target.value)}
          />

          <PillButton type="submit" disabled={busy} className="!mt-4">
            {busy ? 'Sending…' : 'Send reset link'}
          </PillButton>
        </form>
      )}
    </AuthLayout>
  )
}
