import { useState, type FormEvent } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/context'
import { Alert, Button, Card, Field } from '../components/ui'
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
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <h1 className="mb-1 text-xl font-semibold">Sign in</h1>
        <p className="mb-6 text-sm text-slate-500 dark:text-slate-400">
          Seeded account: <code className="text-xs">admin@example.com</code> / <code className="text-xs">password</code>
        </p>

        <form onSubmit={submit} className="space-y-4">
          {error && <Alert kind="error">{error.message}</Alert>}

          <Field
            label="Email"
            type="email"
            value={email}
            autoComplete="email"
            required
            error={error?.fields.email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <Field
            label="Password"
            type="password"
            value={password}
            autoComplete="current-password"
            required
            error={error?.fields.password}
            onChange={(e) => setPassword(e.target.value)}
          />

          <Button type="submit" disabled={busy} className="w-full">
            {busy ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>

        <div className="mt-5 flex justify-between text-sm">
          <Link to="/register" className="text-blue-600 hover:underline dark:text-blue-400">
            Create an account
          </Link>
          <Link to="/forgot-password" className="text-slate-500 hover:underline dark:text-slate-400">
            Forgot password?
          </Link>
        </div>
      </Card>
    </div>
  )
}
