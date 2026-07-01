import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Activity, Boxes, Loader2, MoreVertical, Plus, RefreshCw, Server, Trash2 } from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState } from '@/components/common/misc'
import { StatusBadge, EngineBadge } from '@/components/common/badges'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useToast } from '@/components/ui/use-toast'
import { ServerFormDialog } from '@/components/servers/server-form-dialog'
import { useCheckServer, useDeleteServer, useServers } from '@/api/hooks'
import type { Server as ServerType } from '@/api/types'
import { formatRelativeTime } from '@/lib/utils'

export default function ServersPage() {
  const { data: servers, isLoading } = useServers()
  const { toast } = useToast()
  const checkServer = useCheckServer()
  const deleteServer = useDeleteServer()

  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ServerType | undefined>()
  const [deleting, setDeleting] = useState<ServerType | undefined>()

  function openCreate() {
    setEditing(undefined)
    setFormOpen(true)
  }

  function openEdit(server: ServerType) {
    setEditing(server)
    setFormOpen(true)
  }

  async function handleCheck(server: ServerType) {
    try {
      await checkServer.mutateAsync(server.id)
      toast({ title: 'Server checked', description: server.name })
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Check failed',
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  async function handleDelete() {
    if (!deleting) return
    try {
      await deleteServer.mutateAsync(deleting.id)
      toast({ title: 'Server deleted', description: deleting.name })
      setDeleting(undefined)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Failed to delete server',
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  const addButton = (
    <Button onClick={openCreate}>
      <Plus className="h-4 w-4" />
      Add server
    </Button>
  )

  return (
    <div>
      <PageHeader title="Servers" description="Manage the servers running your inbounds">
        {addButton}
      </PageHeader>

      {isLoading || !servers ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-56" />
          ))}
        </div>
      ) : servers.length === 0 ? (
        <EmptyState
          icon={Server}
          title="No servers yet"
          description="Connect a server over SSH to start managing inbounds."
          action={addButton}
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {servers.map((s) => (
            <Link key={s.id} to={`/servers/${s.id}`} className="group">
              <Card className="h-full transition-colors group-hover:border-primary/40">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <CardTitle className="truncate text-base">{s.name}</CardTitle>
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">
                        {s.geo_country ? `${s.geo_country} · ` : ''}
                        {s.host}
                      </p>
                    </div>
                    <div className="flex items-center gap-1">
                      <StatusBadge status={s.status} />
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            onClick={(e) => {
                              e.preventDefault()
                              e.stopPropagation()
                            }}
                            aria-label="Server actions"
                          >
                            <MoreVertical className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent
                          align="end"
                          onClick={(e) => {
                            e.preventDefault()
                            e.stopPropagation()
                          }}
                        >
                          <DropdownMenuItem
                            onSelect={() => handleCheck(s)}
                            disabled={checkServer.isPending}
                          >
                            <RefreshCw className="h-4 w-4" />
                            Check now
                          </DropdownMenuItem>
                          <DropdownMenuItem onSelect={() => openEdit(s)}>Edit</DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onSelect={() => setDeleting(s)}
                          >
                            <Trash2 className="h-4 w-4" />
                            Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex flex-wrap gap-1.5">
                    {!s.engines || s.engines.length === 0 ? (
                      <span className="text-xs text-muted-foreground">No engines detected</span>
                    ) : (
                      s.engines.map((e) => (
                        <EngineBadge key={e.engine} engine={e.engine} running={e.running} />
                      ))
                    )}
                  </div>
                  <div className="flex items-center justify-between text-sm">
                    <span className="flex items-center gap-1.5 text-muted-foreground">
                      <Boxes className="h-4 w-4" />
                      {s.inbound_count ?? 0} inbound{(s.inbound_count ?? 0) === 1 ? '' : 's'}
                    </span>
                    {s.geo_city && <span className="text-xs text-muted-foreground">{s.geo_city}</span>}
                  </div>
                  <div className="flex items-center gap-1.5 pt-1 text-xs text-muted-foreground">
                    <Activity className="h-3.5 w-3.5" />
                    Synced {formatRelativeTime(s.last_sync_at)}
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}

      <ServerFormDialog open={formOpen} onOpenChange={setFormOpen} server={editing} />

      <Dialog open={!!deleting} onOpenChange={(o) => !o && setDeleting(undefined)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete server</DialogTitle>
            <DialogDescription>
              This will permanently remove <span className="font-medium text-foreground">{deleting?.name}</span> and
              its inbounds from the panel. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleting(undefined)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleteServer.isPending}>
              {deleteServer.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
