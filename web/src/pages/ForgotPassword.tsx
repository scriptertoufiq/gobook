import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import * as authApi from '../api/auth'
import { Alert, Button, Card, Field } from '../components/ui'
import { toFormError } from '../lib/errors'

export function ForgotPassword() {
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
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <h1 className="mb-1 text-xl font-semibold">Reset your password</h1>
        <p className="mb-6 text-sm text-slate-500 dark:text-slate-400">
          We'll email you a link if the address has an account.
        </p>

        {sent ? (
          <Alert kind="success">{sent}</Alert>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            {error && <Alert kind="error">{error}</Alert>}
            <Field
              label="Email"
              type="email"
              value={email}
              required
              autoComplete="email"
              onChange={(e) => setEmail(e.target.value)}
            />
            <Button type="submit" disabled={busy} className="w-full">
              {busy ? 'Sending…' : 'Send reset link'}
            </Button>
          </form>
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
