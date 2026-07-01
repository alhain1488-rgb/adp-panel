import { cn } from '@/lib/utils'

// Brand mark: the "ЧЕРЕМША" meme, served from /public.
export function DisgustingLogo({ className }: { className?: string }) {
  return (
    <img
      src="/cheremsha.webp"
      alt="Absolutely Disgusting Panel"
      className={cn('rounded-lg bg-white object-contain p-0.5', className)}
    />
  )
}
