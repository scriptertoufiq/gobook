import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react'

export function Field({
  label,
  error,
  ...props
}: { label: string; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-medium text-slate-700 dark:text-slate-300">
        {label}
      </span>
      <input
        {...props}
        aria-invalid={error ? true : undefined}
        className={`w-full rounded-lg border px-3 py-2 text-sm outline-none transition
          focus:ring-2 focus:ring-blue-500/40
          dark:bg-slate-900 dark:text-slate-100
          ${
            error
              ? 'border-red-400 dark:border-red-500'
              : 'border-slate-300 focus:border-blue-500 dark:border-slate-700'
          }`}
      />
      {error && <span className="mt-1 block text-xs text-red-600 dark:text-red-400">{error}</span>}
    </label>
  )
}

export function Button({
  children,
  variant = 'primary',
  ...props
}: { variant?: 'primary' | 'ghost' | 'danger' } & ButtonHTMLAttributes<HTMLButtonElement>) {
  const styles = {
    primary: 'bg-blue-600 text-white hover:bg-blue-700 disabled:bg-blue-600/50',
    ghost:
      'border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800',
    danger: 'bg-red-600 text-white hover:bg-red-700 disabled:bg-red-600/50',
  }[variant]

  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium
        transition disabled:cursor-not-allowed ${styles} ${props.className ?? ''}`}
    >
      {children}
    </button>
  )
}

export function Alert({ kind, children }: { kind: 'error' | 'success' | 'info'; children: ReactNode }) {
  const styles = {
    error: 'bg-red-50 text-red-800 dark:bg-red-950/50 dark:text-red-300',
    success: 'bg-green-50 text-green-800 dark:bg-green-950/50 dark:text-green-300',
    info: 'bg-blue-50 text-blue-800 dark:bg-blue-950/50 dark:text-blue-300',
  }[kind]

  return <div className={`rounded-lg px-3 py-2 text-sm ${styles}`}>{children}</div>
}

export function Badge({ children, tone }: { children: ReactNode; tone: 'green' | 'slate' | 'amber' }) {
  const styles = {
    green: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300',
    slate: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
    amber: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300',
  }[tone]

  return <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${styles}`}>{children}</span>
}

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={`rounded-xl border border-slate-200 bg-white p-6 shadow-sm
        dark:border-slate-800 dark:bg-slate-900 ${className}`}
    >
      {children}
    </div>
  )
}

/* ---------------------------------------------------------------------------
 * Auth-screen variants.
 *
 * The signed-in app labels its inputs; the auth screens use placeholder-only
 * pills, which reads cleaner on a short, self-evident form. Kept as separate
 * components rather than props on Field/Button so neither style accumulates
 * conditionals for the other's benefit.
 * ------------------------------------------------------------------------ */

export function PillField({
  error,
  ...props
}: { error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div>
      <input
        {...props}
        aria-invalid={error ? true : undefined}
        className={`w-full rounded-xl border bg-white px-4 py-3.5 text-[15px] outline-none transition
          placeholder:text-slate-500
          focus:border-blue-500 focus:ring-4 focus:ring-blue-500/15
          dark:bg-slate-900 dark:text-slate-100 dark:placeholder:text-slate-500
          ${error ? 'border-red-400 dark:border-red-500' : 'border-slate-300 dark:border-slate-700'}`}
      />
      {error && <p className="mt-1.5 px-1 text-xs text-red-600 dark:text-red-400">{error}</p>}
    </div>
  )
}

export function PillButton({
  children,
  variant = 'primary',
  ...props
}: { variant?: 'primary' | 'outline' } & ButtonHTMLAttributes<HTMLButtonElement>) {
  const styles =
    variant === 'primary'
      ? 'bg-blue-600 text-white hover:bg-blue-700 disabled:bg-blue-600/50'
      : 'border border-blue-600 text-blue-600 hover:bg-blue-50 dark:border-blue-500 dark:text-blue-400 dark:hover:bg-blue-950/40'

  return (
    <button
      {...props}
      className={`w-full rounded-xl px-4 py-3.5 text-[15px] font-semibold transition
        disabled:cursor-not-allowed ${styles} ${props.className ?? ''}`}
    >
      {children}
    </button>
  )
}
