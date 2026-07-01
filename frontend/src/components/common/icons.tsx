// Custom glyphs not available in lucide.

// CheckEngineIcon mimics the "check engine" tell-tale light from a car dashboard
// — an engine block silhouette. Filled to read clearly at small sizes.
export function CheckEngineIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <path d="M22 11h-1V9a1 1 0 0 0-1-1h-3l-2-2h-3v2H9a2 2 0 0 0-2 2v1H5.5a1.5 1.5 0 0 0 0 3H7v1a2 2 0 0 0 2 2h1v2h3v-2h2l2-2h3a1 1 0 0 0 1-1v-2h1a1 1 0 0 0 1-1v-1a1 1 0 0 0-1-1Zm-11 3a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z" />
    </svg>
  )
}
