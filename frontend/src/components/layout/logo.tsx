// Brand mark: a queasy green "absolutely disgusted" face — a nod to the meme,
// drawn inline so it needs no image asset and themes cleanly on any background.
export function DisgustingLogo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 32 32"
      className={className}
      role="img"
      aria-label="Absolutely Disgusting"
    >
      {/* sickly green head */}
      <circle cx="16" cy="16" r="14" fill="#8CB84A" stroke="#5c7d2b" strokeWidth="1.5" />
      {/* queasy cheek blush */}
      <ellipse cx="8.5" cy="19" rx="2.3" ry="1.5" fill="#6f9a34" opacity="0.6" />
      <ellipse cx="23.5" cy="19" rx="2.3" ry="1.5" fill="#6f9a34" opacity="0.6" />

      {/* annoyed brows */}
      <path d="M6.5 9.5 L12.5 11.5" stroke="#33471a" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M25.5 9.5 L19.5 11.5" stroke="#33471a" strokeWidth="1.8" strokeLinecap="round" />

      {/* heavy half-lidded eyes */}
      <path d="M7 13 Q10.5 10.5 14 13" fill="none" stroke="#33471a" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M18 13 Q21.5 10.5 25 13" fill="none" stroke="#33471a" strokeWidth="1.8" strokeLinecap="round" />
      <circle cx="10.6" cy="14" r="1.4" fill="#33471a" />
      <circle cx="21.4" cy="14" r="1.4" fill="#33471a" />

      {/* disgusted wavy grimace */}
      <path
        d="M9.5 21.5 q1.6 -2 3.25 0 q1.6 2 3.25 0 q1.6 -2 3.25 0 q1.6 2 3.25 0"
        fill="none"
        stroke="#33471a"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
