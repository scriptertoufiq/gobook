import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import * as authApi from '../api/auth'
import { useAuth } from '../auth/context'
import { AuthLayout } from '../components/AuthLayout'
import { Alert, PillButton, PillField } from '../components/ui'
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
    <AuthLayout
      title="Create your account"
      back="/login"
      footer={
        <PillButton variant="outline" type="button" onClick={() => navigate('/login')}>
          I already have an account
        </PillButton>
      }
    >
      <form onSubmit={submit} className="space-y-3">
        {error && <Alert kind="error">{error.message}</Alert>}

        <PillField
          placeholder="Full name"
          value={name}
          required
          autoComplete="name"
          error={error?.fields.name}
          onChange={(e) => setName(e.target.value)}
        />
        <PillField
          type="email"
          placeholder="Email address"
          value={email}
          required
          autoComplete="email"
          error={error?.fields.email}
          onChange={(e) => setEmail(e.target.value)}
        />
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

        <p className="px-1 pt-1 text-xs text-slate-500 dark:text-slate-400">
          At least 8 characters.
        </p>

        <PillButton type="submit" disabled={busy} className="!mt-4">
          {busy ? 'Creating account…' : 'Sign up'}
        </PillButton>
      </form>
    </AuthLayout>
  )
}
