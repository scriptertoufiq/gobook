/**
 * The decorative collage on the left of the auth screens.
 *
 * Inline SVG rather than an image file: it stays crisp at any size, adds no
 * network request, and its palette can follow the theme. Purely ornamental, so
 * it is hidden from assistive tech.
 */
export function Illustration({ className = '' }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 520 460"
      className={className}
      role="presentation"
      aria-hidden="true"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#7dd3fc" />
          <stop offset="100%" stopColor="#38bdf8" />
        </linearGradient>
        <linearGradient id="warm" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#fed7aa" />
          <stop offset="100%" stopColor="#fdba74" />
        </linearGradient>
        <filter id="soft" x="-30%" y="-30%" width="160%" height="160%">
          <feDropShadow dx="0" dy="10" stdDeviation="14" floodOpacity="0.14" />
        </filter>
      </defs>

      {/* Back card — a soft abstract shape, tilted away */}
      <g filter="url(#soft)" transform="rotate(-6 165 190)">
        <rect x="60" y="120" width="180" height="150" rx="16" fill="url(#warm)" />
        <path
          d="M104 214c-14-10-10-32 8-36 6-14 28-16 37-4 16-3 28 12 22 26-4 11-17 17-30 15-13 6-27 6-37-1Z"
          fill="#93c5fd"
        />
      </g>

      {/* Tall foreground card — the "story" panel */}
      <g filter="url(#soft)">
        <rect x="258" y="46" width="196" height="330" rx="22" fill="url(#sky)" />
        {/* progress bar */}
        <rect x="278" y="66" width="156" height="5" rx="2.5" fill="#ffffff" opacity="0.45" />
        <rect x="278" y="66" width="96" height="5" rx="2.5" fill="#ffffff" />
        {/* abstract figure */}
        <circle cx="356" cy="150" r="30" fill="#1e3a5f" opacity="0.85" />
        <path d="M312 320c0-38 20-66 44-66s44 28 44 66Z" fill="#f8fafc" />
        <path d="M330 202c-16 8-26 22-24 38l88 4c2-20-10-38-28-44-12-4-24-3-36 2Z" fill="#fb923c" />
        {/* time pill */}
        <rect x="352" y="92" width="86" height="30" rx="15" fill="#4f46e5" />
        <circle cx="370" cy="107" r="7" fill="#ffffff" />
        <rect x="384" y="101" width="42" height="12" rx="6" fill="#ffffff" opacity="0.9" />
        {/* pager dots */}
        <rect x="286" y="340" width="86" height="14" rx="7" stroke="#ffffff" strokeWidth="2.5" />
        <circle cx="392" cy="347" r="7" stroke="#ffffff" strokeWidth="2.5" />
        <circle cx="418" cy="347" r="7" stroke="#ffffff" strokeWidth="2.5" />
      </g>

      {/* Front-left post card */}
      <g filter="url(#soft)">
        <rect x="106" y="238" width="176" height="182" rx="16" fill="#ffffff" />
        <rect x="126" y="262" width="136" height="92" rx="10" fill="#fecdd3" />
        <rect x="146" y="286" width="52" height="44" rx="6" fill="#e11d48" />
        <rect x="206" y="278" width="38" height="56" rx="8" fill="#0ea5e9" />
        <rect x="126" y="370" width="122" height="9" rx="4.5" fill="#e2e8f0" />
        <rect x="126" y="389" width="88" height="9" rx="4.5" fill="#e2e8f0" />
        {/* badge on its corner */}
        <rect x="92" y="224" width="38" height="38" rx="11" fill="#2563eb" />
        <path d="M104 236h14v16l-7-5-7 5Z" fill="#ffffff" />
      </g>

      {/* Floating round badges */}
      <g filter="url(#soft)">
        <circle cx="72" cy="78" r="34" fill="#fbbf24" />
        <circle cx="61" cy="70" r="4.5" fill="#78350f" />
        <circle cx="84" cy="70" r="4.5" fill="#78350f" />
        <path d="M56 86a17 17 0 0 0 32 0Z" fill="#78350f" />
      </g>

      <g filter="url(#soft)">
        <circle cx="470" cy="300" r="34" fill="#ec4899" />
        <path
          d="M470 316c-12-8-18-15-18-22a8.6 8.6 0 0 1 15-5.6 8.6 8.6 0 0 1 15 5.6c0 7-6 14-18 22Z"
          fill="#ffffff"
        />
      </g>

      <g filter="url(#soft)">
        <circle cx="332" cy="398" r="42" fill="#ffffff" />
        <circle cx="332" cy="398" r="37" fill="#a5b4fc" />
        <circle cx="332" cy="388" r="14" fill="#4338ca" />
        <path d="M310 428a22 22 0 0 1 44 0Z" fill="#4338ca" />
      </g>
    </svg>
  )
}
