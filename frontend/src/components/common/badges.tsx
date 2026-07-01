import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { PROTOCOL_LABELS } from '@/api/types'
import type { Protocol, ServerStatus } from '@/api/types'

export function StatusBadge({ status }: { status: ServerStatus }) {
  const map: Record<ServerStatus, { label: string; variant: 'success' | 'destructive' | 'secondary' | 'warning'; dot: string }> = {
    online: { label: 'Online', variant: 'success', dot: 'bg-white/90' },
    offline: { label: 'Offline', variant: 'secondary', dot: 'bg-foreground/70' },
    error: { label: 'Error', variant: 'destructive', dot: 'bg-white/90' },
    unknown: { label: 'Unknown', variant: 'warning', dot: 'bg-black/70' },
  }
  const s = map[status] ?? map.unknown
  return (
    <Badge variant={s.variant} className="gap-1.5">
      <span className={cn('h-1.5 w-1.5 rounded-full', s.dot)} />
      {s.label}
    </Badge>
  )
}

const PROTOCOL_STYLES: Record<Protocol, string> = {
  vless: 'border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400',
  vmess: 'border-violet-500/30 bg-violet-500/10 text-violet-600 dark:text-violet-400',
  trojan: 'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400',
  shadowsocks: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  hysteria2: 'border-pink-500/30 bg-pink-500/10 text-pink-600 dark:text-pink-400',
}

export function ProtocolBadge({ protocol }: { protocol: Protocol }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium',
        PROTOCOL_STYLES[protocol],
      )}
    >
      {PROTOCOL_LABELS[protocol] ?? protocol}
    </span>
  )
}

export function EngineBadge({ engine, running }: { engine: string; running: boolean }) {
  return (
    <Badge variant={running ? 'success' : 'secondary'} className="gap-1.5 font-mono text-[11px]">
      <span className={cn('h-1.5 w-1.5 rounded-full', running ? 'bg-white/90' : 'bg-foreground/70')} />
      {engine === 'hysteria' ? 'hysteria2' : engine}
    </Badge>
  )
}
