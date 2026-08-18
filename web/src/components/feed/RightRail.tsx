import { contacts, friendRequests, sponsored } from '../../data/feed'
import { Avatar } from '../Avatar'
import { DotsIcon, PlusIcon, SearchIcon } from '../icons'

export function RightRail() {
  return (
    <aside className="space-y-5 text-[15px]">
      <section>
        <h2 className="mb-2 px-1 text-[17px] font-semibold text-slate-500 dark:text-slate-400">
          Sponsored
        </h2>
        <div className="space-y-3">
          {sponsored.map((ad) => (
            <a
              key={ad.id}
              href="#"
              className="flex items-center gap-3 rounded-lg p-1 transition
                hover:bg-slate-200/70 dark:hover:bg-slate-800"
            >
              <span
                className={`h-28 w-28 shrink-0 rounded-lg bg-gradient-to-br ${ad.media}`}
                aria-hidden="true"
              />
              <span className="min-w-0">
                <span className="block truncate font-medium">{ad.title}</span>
                <span className="block truncate text-xs text-slate-500 dark:text-slate-400">
                  {ad.domain}
                </span>
              </span>
            </a>
          ))}
        </div>
      </section>

      <hr className="border-slate-300 dark:border-slate-800" />

      <section>
        <div className="mb-2 flex items-center justify-between px-1">
          <h2 className="text-[17px] font-semibold text-slate-500 dark:text-slate-400">
            Friend requests
          </h2>
          <button type="button" className="text-sm text-blue-600 hover:underline dark:text-blue-400">
            See all
          </button>
        </div>

        {friendRequests.map((r) => (
          <div key={r.id} className="flex gap-3 rounded-lg p-1">
            <Avatar name={r.name} size={60} />
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline justify-between gap-2">
                <p className="truncate font-semibold">{r.name}</p>
                <span className="shrink-0 text-xs text-slate-500 dark:text-slate-400">{r.age}</span>
              </div>
              <p className="mb-2 text-xs text-slate-500 dark:text-slate-400">
                Followed by {r.mutual}
              </p>
              <div className="flex gap-2">
                <button
                  type="button"
                  className="flex-1 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-semibold
                    text-white transition hover:bg-blue-700"
                >
                  Confirm
                </button>
                <button
                  type="button"
                  className="flex-1 rounded-md bg-slate-200 px-3 py-1.5 text-sm font-semibold
                    transition hover:bg-slate-300 dark:bg-slate-800 dark:hover:bg-slate-700"
                >
                  Delete
                </button>
              </div>
            </div>
          </div>
        ))}
      </section>

      <hr className="border-slate-300 dark:border-slate-800" />

      <section>
        <div className="mb-1 flex items-center justify-between px-1">
          <h2 className="text-[17px] font-semibold text-slate-500 dark:text-slate-400">Contacts</h2>
          <div className="flex gap-1 text-slate-500">
            <button type="button" aria-label="Search contacts" className="rounded-full p-1.5 hover:bg-slate-200/70 dark:hover:bg-slate-800">
              <SearchIcon size={18} />
            </button>
            <button type="button" aria-label="Contacts options" className="rounded-full p-1.5 hover:bg-slate-200/70 dark:hover:bg-slate-800">
              <DotsIcon size={18} />
            </button>
          </div>
        </div>

        {contacts.map((c) => (
          <button
            key={c.id}
            type="button"
            className="flex w-full items-center gap-3 rounded-lg px-1 py-1.5 text-left font-medium
              transition hover:bg-slate-200/70 dark:hover:bg-slate-800"
          >
            <span className="relative">
              <Avatar name={c.name} size={36} />
              {c.online && (
                <span
                  className="absolute right-0 bottom-0 h-3 w-3 rounded-full border-2
                    border-slate-100 bg-green-500 dark:border-slate-950"
                  aria-label="Online"
                />
              )}
            </span>
            <span className="truncate">{c.name}</span>
          </button>
        ))}

        <button
          type="button"
          className="mt-2 flex w-full items-center gap-3 rounded-lg px-1 py-1.5 text-left
            font-medium transition hover:bg-slate-200/70 dark:hover:bg-slate-800"
        >
          <span
            className="flex h-9 w-9 items-center justify-center rounded-full bg-slate-200
              dark:bg-slate-800"
          >
            <PlusIcon size={18} />
          </span>
          Create group chat
        </button>
      </section>
    </aside>
  )
}
