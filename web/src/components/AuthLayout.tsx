import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Brand } from './Brand'
import { Illustration } from './Illustration'

/**
 * The two-panel shell shared by every unauthenticated screen: brand, artwork
 * and headline on the left, a narrow form column on the right.
 *
 * The left panel is decoration, so it is dropped entirely below `lg` rather
 * than being squeezed — a 320px-wide collage helps nobody. On small screens the
 * form gets the full width and keeps a compact brand mark above it.
 */
export function AuthLayout({
  title,
  back,
  children,
  footer,
}: {
  title: string
  /** Renders the back chevron beside the title, pointing at this route. */
  back?: string
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <div className="min-h-screen bg-white lg:grid lg:grid-cols-[1fr_30rem] dark:bg-slate-950">
      {/* Left: brand, artwork, headline */}
      <section className="relative hidden flex-col justify-between overflow-hidden p-10 lg:flex xl:p-14">
        <Link to="/" aria-label="GoBook home" className="relative z-10 w-fit">
          <Brand />
        </Link>

        <Illustration className="pointer-events-none absolute top-1/2 left-1/2 w-[min(46rem,90%)] -translate-x-1/2 -translate-y-[56%]" />

        <h2 className="relative z-10 max-w-md text-6xl leading-[0.95] font-bold tracking-tight text-slate-900 xl:text-7xl dark:text-white">
          Build the
          <br />
          things
          <br />
          <span className="text-blue-600 dark:text-blue-500">you love.</span>
        </h2>
      </section>

      {/* Right: the form column */}
      <section className="flex items-center justify-center border-slate-200 p-6 lg:border-l dark:border-slate-800">
        <div className="w-full max-w-sm">
          {/* Compact brand for the small screens that lose the left panel. */}
          <div className="mb-8 flex justify-center lg:hidden">
            <Brand size={48} />
          </div>

          <div className="mb-6 flex items-center gap-3">
            {back && (
              <Link
                to={back}
                aria-label="Go back"
                className="-ml-1 rounded-full p-1 text-slate-700 transition hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                  <path
                    d="M12.5 16 6.5 10l6-6"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </Link>
            )}
            <h1 className="text-base font-semibold text-slate-900 dark:text-white">{title}</h1>
          </div>

          {children}

          {footer && (
            <div className="mt-6 border-t border-slate-200 pt-6 dark:border-slate-800">{footer}</div>
          )}
        </div>
      </section>
    </div>
  )
}
