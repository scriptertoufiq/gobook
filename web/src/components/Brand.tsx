/** The GoBook mark — a bookmark glyph in the app's blue. */
export function Brand({ size = 56 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 56 56"
      role="img"
      aria-label="GoBook"
      xmlns="http://www.w3.org/2000/svg"
    >
      <rect width="56" height="56" rx="18" fill="#2563eb" />
      <path
        d="M20 15h16a2 2 0 0 1 2 2v24.6a1.4 1.4 0 0 1-2.2 1.14L28 37.4l-7.8 5.34A1.4 1.4 0 0 1 18 41.6V17a2 2 0 0 1 2-2Z"
        fill="#ffffff"
      />
    </svg>
  )
}
