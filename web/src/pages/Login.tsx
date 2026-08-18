import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { AuthLayout } from '../components/AuthLayout'
import { Alert, PillButton, PillField } from '../components/ui'
import { useAuth } from '../auth/context'
import { toFormError, type FormError } from '../lib/errors'

interface LocationState {
  from?: { pathname: string }
}

export function Login() {
  const { login, user } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<FormError | null>(null)
  const [busy, setBusy] = useState(false)

  if (user) return <Navigate to="/" replace />

  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)

    try {
      await login(email, password)
      // Back to wherever the guard interrupted them, or the dashboard.
      const from = (location.state as LocationState | null)?.from?.pathname
      navigate(from ?? '/', { replace: true })
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthLayout
      title="Log in to GoBook"
      footer={
        <PillButton variant="outline" type="button" onClick={() => navigate('/register')}>
          Create new account
        </PillButton>
      }
    >
      <form onSubmit={submit} className="space-y-3">
        {error && <Alert kind="error">{error.message}</Alert>}

        <PillField
          type="email"
          placeholder="Email address"
          value={email}
          autoComplete="email"
          required
          error={error?.fields.email}
          onChange={(e) => setEmail(e.target.value)}
        />
        <PillField
          type="password"
          placeholder="Password"
          value={password}
          autoComplete="current-password"
          required
          error={error?.fields.password}
          onChange={(e) => setPassword(e.target.value)}
        />

        <PillButton type="submit" disabled={busy} className="!mt-4">
          {busy ? 'Logging in…' : 'Log in'}
        </PillButton>
      </form>

      <button
        type="button"
        onClick={() => navigate('/forgot-password')}
        className="mt-4 block w-full text-center text-sm font-semibold text-slate-800 hover:underline dark:text-slate-200"
      >
        Forgotten password?
      </button>

      <p className="mt-6 text-center text-xs text-slate-500 dark:text-slate-500">
        Seeded account — <code>admin@example.com</code> / <code>password</code>
      </p>
    </AuthLayout>
  )
}
