import { cn } from '@/lib/utils'

// Brand mark: the "ЧЕРЕМША" meme, served from /public.
export function DisgustingLogo({ className }: { className?: string }) {
  return (
    <img
      src="/cheremsha.webp"
      alt="Absolutely Disgusting Panel"
      className={cn('rounded-md bg-white object-cover', className)}
    />
  )
}
