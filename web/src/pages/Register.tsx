import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import * as authApi from '../api/auth'
import { useAuth } from '../auth/context'
import { Alert, Button, Card, Field } from '../components/ui'
import { toFormError, type FormError } from '../lib/errors'

export function Register() {
  const { login, user } = useAuth()
  const navigate = useNavigate()

  const [name, setName] = useState('')
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
      // The backend returns the created user, not a token pair — it does not
      // sign you in. Logging in straight after is a frontend choice.
      await authApi.register(name, email, password)
      await login(email, password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(toFormError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <h1 className="mb-6 text-xl font-semibold">Create an account</h1>

        <form onSubmit={submit} className="space-y-4">
          {error && <Alert kind="error">{error.message}</Alert>}

          <Field
            label="Name"
            value={name}
            required
            autoComplete="name"
            error={error?.fields.name}
            onChange={(e) => setName(e.target.value)}
          />
          <Field
            label="Email"
            type="email"
            value={email}
            required
            autoComplete="email"
            error={error?.fields.email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <Field
            label="Password"
            type="password"
            value={password}
            required
            minLength={8}
            autoComplete="new-password"
            error={error?.fields.password}
            onChange={(e) => setPassword(e.target.value)}
          />

          <Button type="submit" disabled={busy} className="w-full">
            {busy ? 'Creating…' : 'Create account'}
          </Button>
        </form>

        <p className="mt-5 text-sm text-slate-500 dark:text-slate-400">
          Already registered?{' '}
          <Link to="/login" className="text-blue-600 hover:underline dark:text-blue-400">
            Sign in
          </Link>
        </p>
      </Card>
    </div>
  )
}
