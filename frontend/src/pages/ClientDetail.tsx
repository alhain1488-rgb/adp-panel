import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  ArrowLeft,
  Pencil,
  Power,
  Trash2,
  RefreshCw,
  Download,
  QrCode as QrCodeIcon,
  Link2Off,
  Loader2,
  Users,
  ShieldCheck,
} from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState } from '@/components/common/misc'
import { ProtocolBadge } from '@/components/common/badges'
import { CopyButton } from '@/components/common/copy-button'
import { QrCode } from '@/components/common/qr-code'
import { ServerInboundGroup } from '@/components/clients/server-inbound-group'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useToast } from '@/components/ui/use-toast'
import { cn } from '@/lib/utils'
import { useT } from '@/i18n/i18n'
import {
  useClient,
  useClientAmneziaWG,
  useClientLinks,
  useDeleteClient,
  useRotateToken,
  useServers,
  useSetClientInbounds,
  useToggleClient,
  useUpdateClient,
} from '@/api/hooks'
import type { Client, ClientLink } from '@/api/types'

function EditClientDialog({
  client,
  open,
  onOpenChange,
}: {
  client: Client
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { toast } = useToast()
  const t = useT()
  const update = useUpdateClient(client.id)
  const [name, setName] = useState(client.name)
  const [remark, setRemark] = useState(client.remark ?? '')

  useEffect(() => {
    if (open) {
      setName(client.name)
      setRemark(client.remark ?? '')
    }
  }, [open, client.name, client.remark])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return
    try {
      await update.mutateAsync({ name: trimmed, remark: remark.trim() || undefined })
      toast({ title: t('clientDetail.updated') })
      onOpenChange(false)
    } catch {
      toast({ title: t('clientDetail.updateFailed'), variant: 'destructive' })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t('clientDetail.edit.title')}</DialogTitle>
            <DialogDescription>{t('clientDetail.edit.desc')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="edit-name">{t('clientDetail.field.name')}</Label>
              <Input
                id="edit-name"
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-remark">{t('clientDetail.field.remark')}</Label>
              <Input
                id="edit-remark"
                value={remark}
                placeholder={t('clientDetail.field.remarkPlaceholder')}
                onChange={(e) => setRemark(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={update.isPending}
            >
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={update.isPending || !name.trim()}>
              {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function LinkRow({ link }: { link: ClientLink }) {
  const t = useT()
  const [qrOpen, setQrOpen] = useState(false)
  return (
    <>
      <div className="flex items-center gap-3 px-4 py-3">
        <ProtocolBadge protocol={link.protocol} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 text-sm font-medium">
            <span className="truncate">{link.server_name ?? t('clientDetail.server')}</span>
            {link.remark && (
              <span className="truncate text-xs font-normal text-muted-foreground">
                {link.remark}
              </span>
            )}
          </div>
          <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">{link.uri}</p>
        </div>
        <Button variant="outline" size="icon" aria-label={t('common.qrCode')} onClick={() => setQrOpen(true)}>
          <QrCodeIcon className="h-4 w-4" />
        </Button>
        <CopyButton value={link.uri} />
      </div>

      <Dialog open={qrOpen} onOpenChange={setQrOpen}>
        <DialogContent className="sm:max-w-xs">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-base">
              <ProtocolBadge protocol={link.protocol} />
              {link.server_name ?? t('clientDetail.server')}
            </DialogTitle>
            <DialogDescription className="truncate font-mono text-[11px]">
              {link.uri}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col items-center gap-3 pb-2">
            <QrCode text={link.uri} size={220} />
            <CopyButton value={link.uri} size="sm" label={t('clientDetail.copyLink')} />
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

function downloadText(filename: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

// AmneziaWG configs (one per granted AmneziaWG inbound): a downloadable wg-quick
// .conf for the AmneziaWG app plus the vpn:// deep link (copy + QR) for the
// AmneziaVPN app. Rendered only when the client actually has AmneziaWG grants.
function AmneziaWGSection({ clientId, enabled }: { clientId: number; enabled: boolean }) {
  const t = useT()
  const { data } = useClientAmneziaWG(clientId)
  if (!enabled || !data || data.length === 0) return null
  return (
    <div>
      <h3 className="mb-1 text-sm font-medium">{t('clientDetail.awg.title')}</h3>
      <p className="mb-3 text-xs text-muted-foreground">{t('clientDetail.awg.hint')}</p>
      <div className="space-y-3">
        {data.map((cfg) => {
          const vpn = cfg.vpn_link ?? ''
          const conf = cfg.conf ?? ''
          const tag = cfg.tag ?? 'amneziawg'
          return (
            <Card key={cfg.inbound_id} className="p-4">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0 flex-1 space-y-3">
                  <div className="text-sm font-medium">
                    {cfg.server_name} · <span className="font-mono">{tag}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Input
                      readOnly
                      value={vpn}
                      onFocus={(e) => e.currentTarget.select()}
                      className="font-mono text-xs"
                    />
                    <CopyButton value={vpn} label="vpn://" />
                  </div>
                  <Button variant="outline" size="sm" onClick={() => downloadText(`${tag}.conf`, conf)}>
                    <Download className="h-4 w-4" />
                    {t('clientDetail.awg.downloadConf')}
                  </Button>
                </div>
                <div className="shrink-0 self-center">
                  <QrCode text={vpn} size={160} />
                </div>
              </div>
            </Card>
          )
        })}
      </div>
    </div>
  )
}

function ConnectionTab({ client }: { client: Client }) {
  const { toast } = useToast()
  const t = useT()
  const rotate = useRotateToken(client.id)
  const { data: links, isLoading: linksLoading } = useClientLinks(client.id)
  const [confirmRotate, setConfirmRotate] = useState(false)

  const subUrl = client.subscription_url ?? ''

  function handleRotate() {
    rotate.mutate(undefined, {
      onSuccess: () => {
        toast({
          title: t('clientDetail.refreshed.title'),
          description: t('clientDetail.refreshed.desc'),
        })
        setConfirmRotate(false)
      },
      onError: () => toast({ title: t('clientDetail.refreshFailed'), variant: 'destructive' }),
    })
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('clientDetail.subscription')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-6 md:grid-cols-[1fr_auto]">
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="sub-url">{t('clientDetail.subscriptionUrl')}</Label>
                <div className="flex gap-2">
                  <Input
                    id="sub-url"
                    readOnly
                    value={subUrl}
                    className="font-mono text-xs"
                    onFocus={(e) => e.currentTarget.select()}
                  />
                  <CopyButton value={subUrl} />
                </div>
                <p className="text-xs text-muted-foreground">
                  {t('clientDetail.subscriptionHint')}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => setConfirmRotate(true)}>
                  <RefreshCw className="h-4 w-4" />
                  {t('clientDetail.refreshConfig')}
                </Button>
                <Button variant="outline" asChild>
                  <a href={`/api/clients/${client.id}/config`} download>
                    <Download className="h-4 w-4" />
                    {t('clientDetail.downloadConfig')}
                  </a>
                </Button>
              </div>
            </div>
            <div className="flex justify-center md:justify-end">
              {subUrl ? (
                <QrCode text={subUrl} size={200} />
              ) : (
                <div className="text-sm text-muted-foreground">{t('clientDetail.noSubscriptionUrl')}</div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <div>
        <h3 className="mb-3 text-sm font-medium">{t('clientDetail.perInboundLinks')}</h3>
        {linksLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        ) : !client.enabled ? (
          <EmptyState
            icon={Link2Off}
            title={t('clientDetail.disabled.title')}
            description={t('clientDetail.disabled.desc')}
          />
        ) : !links || links.length === 0 ? (
          <EmptyState
            icon={Link2Off}
            title={t('clientDetail.noLinks.title')}
            description={t('clientDetail.noLinks.desc')}
          />
        ) : (
          <Card className="divide-y p-0">
            {links.map((link) => (
              <LinkRow key={link.inbound_id} link={link} />
            ))}
          </Card>
        )}
      </div>

      <AmneziaWGSection clientId={client.id} enabled={client.enabled ?? false} />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('clientDetail.credentials')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">UUID</Label>
            <div className="flex gap-2">
              <Input readOnly value={client.uuid} className="font-mono text-xs" />
              <CopyButton value={client.uuid} />
            </div>
          </div>
          {client.password && (
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">{t('clientDetail.password')}</Label>
              <div className="flex gap-2">
                <Input readOnly value={client.password} className="font-mono text-xs" />
                <CopyButton value={client.password} />
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={confirmRotate} onOpenChange={setConfirmRotate}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('clientDetail.refreshDialog.title')}</DialogTitle>
            <DialogDescription>
              {t('clientDetail.refreshDialog.desc')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmRotate(false)} disabled={rotate.isPending}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleRotate} disabled={rotate.isPending}>
              {rotate.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('clientDetail.refresh')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function AccessTab({ client }: { client: Client }) {
  const { toast } = useToast()
  const t = useT()
  const { data: servers, isLoading } = useServers()
  const setInbounds = useSetClientInbounds(client.id)

  const initial = useMemo(() => new Set(client.inbound_ids ?? []), [client.inbound_ids])
  const [selected, setSelected] = useState<Set<number>>(initial)

  useEffect(() => {
    setSelected(new Set(client.inbound_ids ?? []))
  }, [client.inbound_ids])

  const dirty = useMemo(() => {
    if (selected.size !== initial.size) return true
    for (const id of selected) if (!initial.has(id)) return true
    return false
  }, [selected, initial])

  function toggle(inboundId: number, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (checked) next.add(inboundId)
      else next.delete(inboundId)
      return next
    })
  }

  function handleSave() {
    setInbounds.mutate(Array.from(selected), {
      onSuccess: () => toast({ title: t('clientDetail.accessUpdated') }),
      onError: () => toast({ title: t('clientDetail.accessFailed'), variant: 'destructive' }),
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-muted-foreground">
          {t('clientDetail.accessHint')}
        </p>
        <Button onClick={handleSave} disabled={!dirty || setInbounds.isPending}>
          {setInbounds.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
          {t('clientDetail.saveAccess')}
        </Button>
      </div>

      {isLoading || !servers ? (
        <div className="space-y-3">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-28" />
          ))}
        </div>
      ) : servers.length === 0 ? (
        <EmptyState
          icon={ShieldCheck}
          title={t('clientDetail.noServers.title')}
          description={t('clientDetail.noServers.desc')}
        />
      ) : (
        <div className="space-y-4">
          {servers.map((server) => (
            <ServerInboundGroup
              key={server.id}
              server={server}
              selected={selected}
              onToggle={toggle}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default function ClientDetailPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const navigate = useNavigate()
  const { toast } = useToast()
  const t = useT()

  const { data: client, isLoading } = useClient(id)
  const toggle = useToggleClient(id)
  const del = useDeleteClient()

  const [editOpen, setEditOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  function handleToggle() {
    if (!client) return
    toggle.mutate(!client.enabled, {
      onSuccess: () =>
        toast({ title: client.enabled ? t('clientDetail.toggle.disabled') : t('clientDetail.toggle.enabled') }),
      onError: () => toast({ title: t('clientDetail.actionFailed'), variant: 'destructive' }),
    })
  }

  function handleDelete() {
    del.mutate(id, {
      onSuccess: () => {
        toast({ title: t('clientDetail.deleted') })
        navigate('/clients')
      },
      onError: () => toast({ title: t('clientDetail.deleteFailed'), variant: 'destructive' }),
    })
  }

  return (
    <div>
      <Link
        to="/clients"
        className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        {t('clientDetail.backToClients')}
      </Link>

      {isLoading || !client ? (
        <div className="space-y-6">
          <Skeleton className="h-10 w-64" />
          <Skeleton className="h-9 w-full max-w-xs" />
          <Skeleton className="h-64" />
        </div>
      ) : (
        <>
          <PageHeader title={client.name}>
            <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
              <Pencil className="h-4 w-4" />
              {t('common.edit')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleToggle}
              disabled={toggle.isPending}
            >
              <Power className="h-4 w-4" />
              {client.enabled ? t('common.disable') : t('common.enable')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="text-destructive hover:text-destructive"
              onClick={() => setConfirmDelete(true)}
            >
              <Trash2 className="h-4 w-4" />
              {t('common.delete')}
            </Button>
          </PageHeader>

          <div className="-mt-3 mb-6 flex flex-wrap items-center gap-2">
            <Badge variant={client.enabled ? 'success' : 'secondary'} className="gap-1.5">
              <span
                className={cn(
                  'h-1.5 w-1.5 rounded-full',
                  client.enabled ? 'bg-success-foreground/80' : 'bg-muted-foreground',
                )}
              />
              {client.enabled ? t('common.enabled') : t('common.disabled')}
            </Badge>
            <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
              <Users className="h-3.5 w-3.5" />
              {t('clientDetail.grantedInbounds', { n: client.inbound_ids?.length ?? 0 })}
            </span>
            {client.remark && (
              <>
                <Separator orientation="vertical" className="h-4" />
                <span className="text-sm text-muted-foreground">{client.remark}</span>
              </>
            )}
          </div>

          <Tabs defaultValue="connection">
            <TabsList>
              <TabsTrigger value="connection">{t('clientDetail.tab.connection')}</TabsTrigger>
              <TabsTrigger value="access">{t('clientDetail.tab.access')}</TabsTrigger>
            </TabsList>
            <TabsContent value="connection" className="mt-6">
              <ConnectionTab client={client} />
            </TabsContent>
            <TabsContent value="access" className="mt-6">
              <AccessTab client={client} />
            </TabsContent>
          </Tabs>

          <EditClientDialog client={client} open={editOpen} onOpenChange={setEditOpen} />

          <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>{t('clientDetail.delete.title')}</DialogTitle>
                <DialogDescription>
                  {t('clientDetail.delete.desc', { name: client.name })}
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button
                  variant="ghost"
                  onClick={() => setConfirmDelete(false)}
                  disabled={del.isPending}
                >
                  {t('common.cancel')}
                </Button>
                <Button variant="destructive" onClick={handleDelete} disabled={del.isPending}>
                  {del.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                  {t('common.delete')}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </>
      )}
    </div>
  )
}
