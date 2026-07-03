import { useState } from 'react'
import {
  Activity,
  AlertTriangle,
  ChevronDown,
  Clock,
  EthernetPort,
  HardDriveDownload,
  Loader2,
  MoreVertical,
  Pencil,
  Plus,
  RefreshCw,
  RotateCw,
  Trash2,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { useMediaQuery } from '@/lib/use-media-query'
import { EmptyState, UsageBar } from '@/components/common/misc'
import { CheckEngineIcon } from '@/components/common/icons'
import { EngineBadge, ProtocolBadge, StatusBadge } from '@/components/common/badges'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
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
import { useT, useRelTime } from '@/i18n/i18n'
import { ServerFormDialog } from './server-form-dialog'
import { InboundFormDialog } from './inbound-form-dialog'
import {
  useCheckServer,
  useDeleteInbound,
  useDeleteServer,
  useInstallServer,
  useRestartEngine,
  useServerInbounds,
  useServerStats,
  useUpdateInbound,
} from '@/api/hooks'
import type { Inbound, Server } from '@/api/types'

// Provisioning (engine install) indicator — SPEC §5.1.
function ProvisionIndicator({ status }: { status: NonNullable<Server['provision_status']> }) {
  const t = useT()
  if (status === 'installing' || status === 'pending') {
    return (
      <Badge variant="warning" className="gap-1.5">
        <Loader2 className="h-3 w-3 animate-spin" />
        {t('serverCard.installingEngines')}
      </Badge>
    )
  }
  if (status === 'failed') {
    return (
      <Badge variant="destructive" className="gap-1.5">
        <AlertTriangle className="h-3 w-3" />
        {t('serverCard.installFailed')}
      </Badge>
    )
  }
  return null
}

// MetricTile is a compact, fixed-shape stat used inside the expanded card so the
// tiles stay uniform in the narrow grid cell.
function MetricTile({
  label,
  value,
  icon: Icon,
  hint,
}: {
  label: string
  value: React.ReactNode
  icon: React.ComponentType<{ className?: string }>
  hint?: string
}) {
  return (
    <div className="flex items-center justify-between gap-2 rounded-lg border bg-card p-3">
      <div className="min-w-0">
        <p className="truncate text-xs text-muted-foreground">{label}</p>
        <p className="truncate text-lg font-semibold leading-tight tracking-tight">{value}</p>
        {hint && <p className="truncate text-[11px] text-muted-foreground">{hint}</p>}
      </div>
      <div className="shrink-0 rounded-md bg-primary/10 p-2 text-primary">
        <Icon className="h-4 w-4" />
      </div>
    </div>
  )
}

function formatUptime(seconds?: number): string {
  if (seconds == null || seconds <= 0) return '—'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

export function ServerCard({ server }: { server: Server }) {
  // Desktop (md+, where cards sit in a multi-column grid): open details in a modal
  // so the grid never reflows. Mobile (single column): expand inline in place.
  const isDesktop = useMediaQuery('(min-width: 768px)')
  const [expanded, setExpanded] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const isOpen = isDesktop ? modalOpen : expanded
  const toggleOpen = () => (isDesktop ? setModalOpen((v) => !v) : setExpanded((v) => !v))

  const { toast } = useToast()
  const t = useT()
  const rel = useRelTime()
  const checkServer = useCheckServer()
  const restartEngine = useRestartEngine(server.id)
  const deleteServer = useDeleteServer()
  const installServer = useInstallServer()

  async function handleInstall() {
    try {
      await installServer.mutateAsync(server.id)
      toast({ title: t('serverCard.installingEngines'), description: server.name })
    } catch (err) {
      toast({ variant: 'destructive', title: t('serverCard.installFailed'), description: errMsg(err) })
    }
  }

  async function handleCheck() {
    try {
      await checkServer.mutateAsync(server.id)
      toast({ title: t('serverCard.checked'), description: server.name })
    } catch (err) {
      toast({ variant: 'destructive', title: t('serverCard.checkFailed'), description: errMsg(err) })
    }
  }

  async function handleRestart(engine?: string) {
    try {
      await restartEngine.mutateAsync(engine)
      toast({ title: engine ? t('serverCard.restartedEngine', { engine }) : t('serverCard.restartedAll') })
    } catch (err) {
      toast({ variant: 'destructive', title: t('serverCard.restartFailed'), description: errMsg(err) })
    }
  }

  async function handleDelete() {
    try {
      await deleteServer.mutateAsync(server.id)
      toast({ title: t('serverCard.deleted'), description: server.name })
      setDeleteOpen(false)
    } catch (err) {
      toast({ variant: 'destructive', title: t('serverCard.deleteFailed'), description: errMsg(err) })
    }
  }

  const detail = (
    <ServerCardDetail
      server={server}
      onEdit={() => setEditOpen(true)}
      onDelete={() => setDeleteOpen(true)}
      onCheck={handleCheck}
      onRestart={handleRestart}
      onReinstall={handleInstall}
      checking={checkServer.isPending}
      restarting={restartEngine.isPending}
      installing={installServer.isPending}
    />
  )

  return (
    <Card className={cn('overflow-hidden transition-colors', isOpen && 'border-primary/40')}>
      {/* Header — clicking expands inline (mobile) or opens the modal (desktop) */}
      <button
        type="button"
        onClick={toggleOpen}
        className="flex w-full items-start justify-between gap-2 p-5 text-left"
        aria-expanded={isOpen}
      >
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="truncate font-semibold">{server.name}</span>
            <StatusBadge status={server.status} />
            {server.provision_status && server.provision_status !== 'installed' && (
              <ProvisionIndicator status={server.provision_status} />
            )}
          </div>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            {server.geo_country ? `${server.geo_country} · ` : ''}
            {server.host}
          </p>
          <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5">
            {server.engines && server.engines.length > 0 ? (
              server.engines.map((e) => <EngineBadge key={e.engine} engine={e.engine} running={e.running} />)
            ) : (
              <span className="text-xs text-muted-foreground">{t('serverCard.noEngines')}</span>
            )}
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <EthernetPort className="h-3.5 w-3.5" />
              {server.inbound_count ?? 0}
            </span>
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <Activity className="h-3.5 w-3.5" />
              {rel(server.last_sync_at)}
            </span>
          </div>
        </div>
        <ChevronDown
          className={cn('h-5 w-5 shrink-0 text-muted-foreground transition-transform', isOpen && 'rotate-180')}
        />
      </button>

      {/* Mobile: inline expansion in place */}
      {!isDesktop && expanded && <div className="border-t px-5 pb-5 pt-5">{detail}</div>}

      {/* Desktop: details in a modal so the server grid never reflows */}
      {isDesktop && (
        <Dialog open={modalOpen} onOpenChange={setModalOpen}>
          <DialogContent className="max-h-[85vh] max-w-3xl overflow-y-auto">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                {server.name}
                <StatusBadge status={server.status} />
              </DialogTitle>
              <DialogDescription>
                {server.geo_country ? `${server.geo_country} · ` : ''}
                {server.host}
              </DialogDescription>
            </DialogHeader>
            {detail}
          </DialogContent>
        </Dialog>
      )}

      <ServerFormDialog open={editOpen} onOpenChange={setEditOpen} server={server} />

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('serverCard.delete.title')}</DialogTitle>
            <DialogDescription>
              {t('serverCard.delete.descBefore')}{' '}
              <span className="font-medium text-foreground">{server.name}</span>{' '}
              {t('serverCard.delete.descAfter')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleteServer.isPending}>
              {deleteServer.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

// Expanded body — only mounted when the card is open, so stats/inbounds are
// fetched lazily per server on demand.
function ServerCardDetail({
  server,
  onEdit,
  onDelete,
  onCheck,
  onRestart,
  onReinstall,
  checking,
  restarting,
  installing,
}: {
  server: Server
  onEdit: () => void
  onDelete: () => void
  onCheck: () => void
  onRestart: (engine?: string) => void
  onReinstall: () => void
  checking: boolean
  restarting: boolean
  installing: boolean
}) {
  const id = server.id
  const { data: stats } = useServerStats(id)
  const { data: inbounds, isLoading: inboundsLoading } = useServerInbounds(id)
  const { toast } = useToast()
  const t = useT()
  const updateInbound = useUpdateInbound(id)
  const deleteInbound = useDeleteInbound(id)

  const [inboundFormOpen, setInboundFormOpen] = useState(false)
  const [editingInbound, setEditingInbound] = useState<Inbound | undefined>()
  const [deletingInbound, setDeletingInbound] = useState<Inbound | undefined>()
  const [togglingId, setTogglingId] = useState<number | null>(null)

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
      toast({ title: enabled ? t('serverCard.inbound.enabled') : t('serverCard.inbound.disabled'), description: inbound.tag })
    } catch (err) {
      toast({ variant: 'destructive', title: t('serverCard.inbound.updateFailed'), description: errMsg(err) })
    } finally {
      setTogglingId(null)
    }
  }

  async function handleDeleteInbound() {
    if (!deletingInbound) return
    try {
      await deleteInbound.mutateAsync(deletingInbound.id)
      toast({ title: t('serverCard.inbound.deleted'), description: deletingInbound.tag })
      setDeletingInbound(undefined)
    } catch (err) {
      toast({ variant: 'destructive', title: t('serverCard.inbound.deleteFailed'), description: errMsg(err) })
    }
  }

  const details = [
    server.ip,
    server.geo_city && server.geo_country ? `${server.geo_city}, ${server.geo_country}` : server.geo_country,
    server.geo_asn,
  ].filter(Boolean)

  return (
    <div className="space-y-5">
      {/* Server actions */}
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="outline" size="sm" onClick={onCheck} disabled={checking}>
          {checking ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          {t('serverCard.check')}
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" disabled={restarting}>
              {restarting ? <Loader2 className="h-4 w-4 animate-spin" /> : <RotateCw className="h-4 w-4" />}
              {t('serverCard.restart')}
              <ChevronDown className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuItem onSelect={() => onRestart()}>{t('serverCard.allEngines')}</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => onRestart('xray')}>xray</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => onRestart('hysteria')}>hysteria2</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button variant="outline" size="sm" onClick={onEdit}>
          <Pencil className="h-4 w-4" />
          {t('common.edit')}
        </Button>
        <Button variant="outline" size="sm" onClick={onReinstall} disabled={installing}>
          {installing ? <Loader2 className="h-4 w-4 animate-spin" /> : <HardDriveDownload className="h-4 w-4" />}
          {t('serverCard.reinstallEngines')}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="text-destructive hover:text-destructive"
          onClick={onDelete}
        >
          <Trash2 className="h-4 w-4" />
          {t('common.delete')}
        </Button>
      </div>

      {details.length > 0 && (
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
          {details.map((d, i) => (
            <span key={i} className="flex items-center gap-2">
              {i > 0 && <span className="text-muted-foreground/40">·</span>}
              {d}
            </span>
          ))}
        </div>
      )}

      {server.provision_status && server.provision_status !== 'installed' && (
        <div className="flex items-start gap-3 rounded-md border border-warning/40 bg-warning/5 p-3">
          {server.provision_status === 'failed' ? (
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
          ) : (
            <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-warning" />
          )}
          <div>
            <p className="text-sm font-medium">
              {server.provision_status === 'failed'
                ? t('serverCard.provision.failed')
                : t('serverCard.provision.installing')}
            </p>
            {server.provision_error && (
              <p className="mt-1 break-words text-xs text-muted-foreground">{server.provision_error}</p>
            )}
            {server.provision_status !== 'failed' && (
              <p className="mt-1 text-xs text-muted-foreground">
                {t('serverCard.provision.notPushed')}
              </p>
            )}
          </div>
        </div>
      )}

      {server.last_sync_error && (
        <div className="flex items-start gap-3 rounded-md border border-destructive/40 bg-destructive/5 p-3">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
          <div>
            <p className="text-sm font-medium text-destructive">{t('serverCard.lastSyncFailed')}</p>
            <p className="mt-1 break-words text-xs text-muted-foreground">{server.last_sync_error}</p>
          </div>
        </div>
      )}

      {/* Metrics: usage bars full width, then three equal-size tiles */}
      <div className="space-y-3">
        <Card>
          <CardContent className="space-y-3 p-4">
            <UsageBar label={t('serverCard.cpu')} percent={stats?.cpu_percent ?? 0} />
            <UsageBar
              label={t('serverCard.ram')}
              percent={stats?.mem_percent ?? 0}
              detail={
                stats ? `${(stats.mem_used_mb / 1024).toFixed(1)}/${(stats.mem_total_mb / 1024).toFixed(1)} GB` : undefined
              }
            />
            <UsageBar
              label={t('serverCard.disk')}
              percent={stats?.disk_percent ?? 0}
              detail={stats ? `${stats.disk_used_gb.toFixed(0)}/${stats.disk_total_gb.toFixed(0)} GB` : undefined}
            />
          </CardContent>
        </Card>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <MetricTile label={t('serverCard.uptime')} value={formatUptime(stats?.uptime_seconds)} icon={Clock} />
          <MetricTile label={t('serverCard.inbounds')} value={server.inbound_count ?? inbounds?.length ?? 0} icon={EthernetPort} />
          <MetricTile
            label={t('serverCard.engines')}
            value={server.engines?.filter((e) => e.running).length ?? 0}
            icon={CheckEngineIcon}
            hint={t('serverCard.detected', { n: server.engines?.length ?? 0 })}
          />
        </div>
      </div>

      {/* Engines: equal-width tiles */}
      {server.engines && server.engines.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2">
          {server.engines.map((e) => (
            <div key={e.engine} className="flex min-w-0 items-center gap-3 rounded-md border px-3 py-2">
              <EngineBadge engine={e.engine} running={e.running} />
              <div className="min-w-0 truncate text-xs text-muted-foreground">
                {e.version && <span className="font-mono">{e.version}</span>}
                {e.version && e.service_name && ' · '}
                {e.service_name && <span className="font-mono">{e.service_name}</span>}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Inbounds */}
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{t('serverCard.inbounds')}</h3>
        <Button
          size="sm"
          onClick={() => {
            setEditingInbound(undefined)
            setInboundFormOpen(true)
          }}
        >
          <Plus className="h-4 w-4" />
          {t('serverCard.addInbound')}
        </Button>
      </div>

      {inboundsLoading || !inbounds ? (
        <Skeleton className="h-32" />
      ) : inbounds.length === 0 ? (
        <EmptyState icon={EthernetPort} title={t('serverCard.noInbounds.title')} description={t('serverCard.noInbounds.desc')} />
      ) : (
        <div className="space-y-2">
          {inbounds.map((ib) => (
            <div key={ib.id} className="flex items-center gap-3 rounded-md border p-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate font-medium">{ib.tag}</span>
                  <ProtocolBadge protocol={ib.protocol} />
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                  <span className="font-mono tabular-nums">:{ib.port}</span>
                  <span className="tabular-nums">{t('serverCard.clientsCount', { n: ib.client_count ?? 0 })}</span>
                  {ib.remark && <span className="truncate">{ib.remark}</span>}
                </div>
              </div>
              <Switch
                checked={ib.enabled}
                disabled={togglingId === ib.id}
                onCheckedChange={(v) => handleToggleInbound(ib, v)}
                aria-label={t('serverCard.toggleInbound')}
              />
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" aria-label={t('serverCard.inboundActions')}>
                    <MoreVertical className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    onSelect={() => {
                      setEditingInbound(ib)
                      setInboundFormOpen(true)
                    }}
                  >
                    <Pencil className="h-4 w-4" />
                    {t('common.edit')}
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onSelect={() => setDeletingInbound(ib)}
                  >
                    <Trash2 className="h-4 w-4" />
                    {t('common.delete')}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          ))}
        </div>
      )}

      <InboundFormDialog
        serverId={id}
        open={inboundFormOpen}
        onOpenChange={setInboundFormOpen}
        inbound={editingInbound}
      />

      <Dialog open={!!deletingInbound} onOpenChange={(o) => !o && setDeletingInbound(undefined)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('serverCard.deleteInbound.title')}</DialogTitle>
            <DialogDescription>
              {t('serverCard.deleteInbound.descBefore')}{' '}
              <span className="font-medium text-foreground">{deletingInbound?.tag}</span>{' '}
              {t('serverCard.deleteInbound.descAfter')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeletingInbound(undefined)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={handleDeleteInbound} disabled={deleteInbound.isPending}>
              {deleteInbound.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function errMsg(err: unknown): string | undefined {
  return err instanceof Error ? err.message : undefined
}
