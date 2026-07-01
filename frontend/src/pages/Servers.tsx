import { useState } from 'react'
import { Plus } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { ServerCard } from '@/components/servers/server-card'
import { ServerFormDialog } from '@/components/servers/server-form-dialog'
import { useServers } from '@/api/hooks'

export default function ServersPage() {
  const { data: servers, isLoading } = useServers()
  const [createOpen, setCreateOpen] = useState(false)

  return (
    <div>
      {isLoading || !servers ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40" />
          ))}
        </div>
      ) : (
        <div className="grid items-start gap-4 md:grid-cols-2 xl:grid-cols-3">
          {servers.map((s) => (
            <ServerCard key={s.id} server={s} />
          ))}

          {/* Add-server card: dashed outline with a plus, after the last server */}
          <button
            type="button"
            onClick={() => setCreateOpen(true)}
            aria-label="Add server"
            className="flex min-h-[7.5rem] flex-col items-center justify-center gap-2 self-stretch rounded-xl border-2 border-dashed border-border text-muted-foreground transition-colors hover:border-primary/50 hover:text-primary"
          >
            <Plus className="h-8 w-8" />
            <span className="text-sm font-medium">Add server</span>
          </button>
        </div>
      )}

      <ServerFormDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}
