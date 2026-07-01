import { Server as ServerIcon } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import { ProtocolBadge } from '@/components/common/badges'
import { useServerInbounds } from '@/api/hooks'
import { cn } from '@/lib/utils'
import type { Server } from '@/api/types'

// Rendered once per server so the useServerInbounds hook is called from a stable
// component (never in a top-level loop).
export function ServerInboundGroup({
  server,
  selected,
  onToggle,
}: {
  server: Server
  selected: Set<number>
  onToggle: (inboundId: number, checked: boolean) => void
}) {
  const { data: inbounds, isLoading } = useServerInbounds(server.id)

  return (
    <div className="rounded-lg border">
      <div className="flex items-center gap-2 border-b px-4 py-2.5">
        <ServerIcon className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-medium">{server.name}</span>
        {server.geo_country && (
          <span className="text-xs text-muted-foreground">{server.geo_country}</span>
        )}
      </div>

      {isLoading || !inbounds ? (
        <div className="space-y-2 p-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-8" />
          ))}
        </div>
      ) : inbounds.length === 0 ? (
        <p className="px-4 py-3 text-sm text-muted-foreground">No inbounds on this server.</p>
      ) : (
        <ul className="divide-y">
          {inbounds.map((inbound) => {
            const checked = selected.has(inbound.id)
            const id = `inbound-${inbound.id}`
            return (
              <li key={inbound.id}>
                <label
                  htmlFor={id}
                  className="flex cursor-pointer items-center gap-3 px-4 py-2.5 hover:bg-muted/40"
                >
                  <Checkbox
                    id={id}
                    checked={checked}
                    onCheckedChange={(v) => onToggle(inbound.id, v === true)}
                  />
                  <ProtocolBadge protocol={inbound.protocol} />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">{inbound.tag}</span>
                  <span className="font-mono text-xs text-muted-foreground tabular-nums">
                    :{inbound.port}
                  </span>
                  <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                    <span
                      className={cn(
                        'h-1.5 w-1.5 rounded-full',
                        inbound.enabled ? 'bg-success' : 'bg-muted-foreground',
                      )}
                    />
                    {inbound.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </label>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
