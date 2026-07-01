import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  AlertTriangle,
  ArrowLeft,
  Boxes,
  ChevronDown,
  Clock,
  Loader2,
  MoreVertical,
  Pencil,
  Plus,
  RefreshCw,
  RotateCw,
  Trash2,
} from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState, StatCard, UsageBar } from '@/components/common/misc'
import { EngineBadge, ProtocolBadge, StatusBadge } from '@/components/common/badges'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
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
import { InboundFormDialog } from '@/components/servers/inbound-form-dialog'
import {
  useCheckServer,
  useDeleteInbound,
  useRestartEngine,
  useServer,
  useServerInbounds,
  useServerStats,
  useUpdateInbound,
} from '@/api/hooks'
import type { Inbound } from '@/api/types'

function formatUptime(seconds?: number): string {
  if (seconds == null || seconds <= 0) return '—'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

export default function ServerDetailPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)

  const { data: server, isLoading: serverLoading } = useServer(id)
  const { data: inbounds, isLoading: inboundsLoading } = useServerInbounds(id)
  const { data: stats } = useServerStats(id)

  const { toast } = useToast()
  const checkServer = useCheckServer()
  const restartEngine = useRestartEngine(id)
  const updateInbound = useUpdateInbound(id)
  const deleteInbound = useDeleteInbound(id)

  const [editServerOpen, setEditServerOpen] = useState(false)
  const [inboundFormOpen, setInboundFormOpen] = useState(false)
  const [editingInbound, setEditingInbound] = useState<Inbound | undefined>()
  const [deletingInbound, setDeletingInbound] = useState<Inbound | undefined>()
  const [togglingId, setTogglingId] = useState<number | null>(null)

  async function handleCheck() {
    if (!server) return
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

  async function handleRestart(engine?: string) {
    try {
      await restartEngine.mutateAsync(engine)
      toast({ title: engine ? `Restarted ${engine}` : 'Restarted all engines' })
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Restart failed',
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  async function handleToggleInbound(inbound: Inbound, enabled: boolean) {
    setTogglingId(inbound.id)
    try {
      await updateInbound.mutateAsync({
        id: inbound.id,
        input: {
          tag: inbound.tag,
          protocol: inbound.protocol,
          listen: inbound.listen ?? '0.0.0.0',
          port: inbound.port,
          remark: inbound.remark,
          enabled,
          settings: inbound.settings,
          stream_settings: inbound.stream_settings,
        },
      })
      toast({ title: enabled ? 'Inbound enabled' : 'Inbound disabled', description: inbound.tag })
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Failed to update inbound',
        description: err instanceof Error ? err.message : undefined,
      })
    } finally {
      setTogglingId(null)
    }
  }

  async function handleDeleteInbound() {
    if (!deletingInbound) return
    try {
      await deleteInbound.mutateAsync(deletingInbound.id)
      toast({ title: 'Inbound deleted', description: deletingInbound.tag })
      setDeletingInbound(undefined)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Failed to delete inbound',
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  function openCreateInbound() {
    setEditingInbound(undefined)
    setInboundFormOpen(true)
  }

  function openEditInbound(inbound: Inbound) {
    setEditingInbound(inbound)
    setInboundFormOpen(true)
  }

  const backLink = (
    <Link
      to="/servers"
      className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft className="h-4 w-4" />
      Servers
    </Link>
  )

  if (serverLoading || !server) {
    return (
      <div>
        {backLink}
        <Skeleton className="mb-6 h-10 w-64" />
        <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <Skeleton className="h-64" />
      </div>
    )
  }

  const details = [
    server.ip,
    server.geo_city && server.geo_country
      ? `${server.geo_city}, ${server.geo_country}`
      : server.geo_country,
    server.geo_asn,
  ].filter(Boolean)

  return (
    <div>
      {backLink}

      <PageHeader title={server.name} description={server.host}>
        <Button variant="outline" onClick={handleCheck} disabled={checkServer.isPending}>
          {checkServer.isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <RefreshCw className="h-4 w-4" />
          )}
          Check
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" disabled={restartEngine.isPending}>
              {restartEngine.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RotateCw className="h-4 w-4" />
              )}
              Restart
              <ChevronDown className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => handleRestart()}>All engines</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => handleRestart('xray')}>xray</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => handleRestart('hysteria')}>hysteria2</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button variant="outline" onClick={() => setEditServerOpen(true)}>
          <Pencil className="h-4 w-4" />
          Edit
        </Button>
      </PageHeader>

      <div className="mb-6 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
        <StatusBadge status={server.status} />
        {details.map((d, i) => (
          <span key={i} className="flex items-center gap-3">
            {i > 0 && <span className="text-muted-foreground/40">·</span>}
            {d}
          </span>
        ))}
      </div>

      {server.last_sync_error && (
        <Card className="mb-6 border-destructive/40 bg-destructive/5">
          <CardContent className="flex items-start gap-3 p-4">
            <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-destructive" />
            <div>
              <p className="text-sm font-medium text-destructive">Last sync failed</p>
              <p className="mt-1 break-words text-sm text-muted-foreground">{server.last_sync_error}</p>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="mb-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardContent className="space-y-3 p-5">
            <UsageBar label="CPU" percent={stats?.cpu_percent ?? 0} />
            <UsageBar
              label="RAM"
              percent={stats?.mem_percent ?? 0}
              detail={
                stats?.mem_used_mb != null && stats?.mem_total_mb != null
                  ? `${(stats.mem_used_mb / 1024).toFixed(1)}/${(stats.mem_total_mb / 1024).toFixed(1)} GB`
                  : undefined
              }
            />
            <UsageBar
              label="Disk"
              percent={stats?.disk_percent ?? 0}
              detail={
                stats?.disk_used_gb != null && stats?.disk_total_gb != null
                  ? `${stats.disk_used_gb.toFixed(0)}/${stats.disk_total_gb.toFixed(0)} GB`
                  : undefined
              }
            />
          </CardContent>
        </Card>
        <StatCard label="Uptime" value={formatUptime(stats?.uptime_seconds)} icon={Clock} />
        <StatCard label="Inbounds" value={server.inbound_count ?? inbounds?.length ?? 0} icon={Boxes} />
        <StatCard
          label="Engines"
          value={server.engines?.filter((e) => e.running).length ?? 0}
          icon={RotateCw}
          hint={`${server.engines?.length ?? 0} detected`}
        />
      </div>

      <Card className="mb-6">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Engines</CardTitle>
        </CardHeader>
        <CardContent>
          {!server.engines || server.engines.length === 0 ? (
            <p className="text-sm text-muted-foreground">No engines detected on this server.</p>
          ) : (
            <div className="flex flex-wrap gap-4">
              {server.engines.map((e) => (
                <div
                  key={e.engine}
                  className="flex items-center gap-3 rounded-md border px-3 py-2"
                >
                  <EngineBadge engine={e.engine} running={e.running} />
                  <div className="text-xs text-muted-foreground">
                    {e.version && <span className="font-mono">{e.version}</span>}
                    {e.version && e.service_name && ' · '}
                    {e.service_name && <span className="font-mono">{e.service_name}</span>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold">Inbounds</h2>
        <Button onClick={openCreateInbound}>
          <Plus className="h-4 w-4" />
          Add inbound
        </Button>
      </div>

      {inboundsLoading || !inbounds ? (
        <Skeleton className="h-48" />
      ) : inbounds.length === 0 ? (
        <EmptyState
          icon={Boxes}
          title="No inbounds yet"
          description="Add an inbound to expose a protocol on this server."
          action={
            <Button onClick={openCreateInbound}>
              <Plus className="h-4 w-4" />
              Add inbound
            </Button>
          }
        />
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Tag</TableHead>
                <TableHead>Protocol</TableHead>
                <TableHead>Port</TableHead>
                <TableHead>Clients</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead className="w-[1%] text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {inbounds.map((ib) => (
                <TableRow key={ib.id}>
                  <TableCell className="font-medium">
                    {ib.tag}
                    {ib.remark && (
                      <span className="ml-2 text-xs font-normal text-muted-foreground">{ib.remark}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <ProtocolBadge protocol={ib.protocol} />
                  </TableCell>
                  <TableCell className="font-mono text-sm tabular-nums">{ib.port}</TableCell>
                  <TableCell className="tabular-nums">{ib.client_count ?? 0}</TableCell>
                  <TableCell>
                    <Switch
                      checked={ib.enabled}
                      disabled={togglingId === ib.id}
                      onCheckedChange={(v) => handleToggleInbound(ib, v)}
                      aria-label="Toggle inbound"
                    />
                  </TableCell>
                  <TableCell className="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8" aria-label="Inbound actions">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onSelect={() => openEditInbound(ib)}>
                          <Pencil className="h-4 w-4" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onSelect={() => setDeletingInbound(ib)}
                        >
                          <Trash2 className="h-4 w-4" />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <ServerFormDialog open={editServerOpen} onOpenChange={setEditServerOpen} server={server} />
      <InboundFormDialog
        serverId={id}
        open={inboundFormOpen}
        onOpenChange={setInboundFormOpen}
        inbound={editingInbound}
      />

      <Dialog open={!!deletingInbound} onOpenChange={(o) => !o && setDeletingInbound(undefined)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete inbound</DialogTitle>
            <DialogDescription>
              This will permanently remove{' '}
              <span className="font-medium text-foreground">{deletingInbound?.tag}</span> from this server. This
              action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeletingInbound(undefined)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDeleteInbound} disabled={deleteInbound.isPending}>
              {deleteInbound.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
