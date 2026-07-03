import { useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useT } from '@/i18n/i18n'

export function CopyButton({
  value,
  size = 'icon',
  label,
  className,
}: {
  value: string
  size?: 'icon' | 'sm'
  label?: string
  className?: string
}) {
  const t = useT()
  const [copied, setCopied] = useState(false)
  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
    } catch {
      /* clipboard may be unavailable */
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <Button variant="outline" size={size} onClick={copy} className={cn(className)} aria-label={t('common.copy')}>
      {copied ? <Check className="h-4 w-4 text-success" /> : <Copy className="h-4 w-4" />}
      {label && <span>{copied ? t('common.copied') : label}</span>}
    </Button>
  )
}
