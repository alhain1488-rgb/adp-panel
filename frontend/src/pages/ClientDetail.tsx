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
import {
  useClient,
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
      toast({ title: 'Client updated' })
      onOpenChange(false)
    } catch {
      toast({ title: 'Could not update client', variant: 'destructive' })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Edit client</DialogTitle>
            <DialogDescription>Update the display name and an optional note.</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="edit-name">Name</Label>
              <Input
                id="edit-name"
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-remark">Remark</Label>
              <Input
                id="edit-remark"
                value={remark}
                placeholder="Optional note"
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
              Cancel
            </Button>
            <Button type="submit" disabled={update.isPending || !name.trim()}>
              {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function LinkRow({ link }: { link: ClientLink }) {
  const [qrOpen, setQrOpen] = useState(false)
  return (
    <>
      <div className="flex items-center gap-3 px-4 py-3">
        <ProtocolBadge protocol={link.protocol} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 text-sm font-medium">
            <span className="truncate">{link.server_name ?? 'Server'}</span>
            {link.remark && (
              <span className="truncate text-xs font-normal text-muted-foreground">
                {link.remark}
              </span>
            )}
          </div>
          <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">{link.uri}</p>
        </div>
        <Button variant="outline" size="icon" aria-label="Show QR" onClick={() => setQrOpen(true)}>
          <QrCodeIcon className="h-4 w-4" />
        </Button>
        <CopyButton value={link.uri} />
      </div>

      <Dialog open={qrOpen} onOpenChange={setQrOpen}>
        <DialogContent className="sm:max-w-xs">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-base">
              <ProtocolBadge protocol={link.protocol} />
              {link.server_name ?? 'Server'}
            </DialogTitle>
            <DialogDescription className="truncate font-mono text-[11px]">
              {link.uri}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col items-center gap-3 pb-2">
            <QrCode text={link.uri} size={220} />
            <CopyButton value={link.uri} size="sm" label="Copy link" />
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

function ConnectionTab({ client }: { client: Client }) {
  const { toast } = useToast()
  const rotate = useRotateToken(client.id)
  const { data: links, isLoading: linksLoading } = useClientLinks(client.id)
  const [confirmRotate, setConfirmRotate] = useState(false)

  const subUrl = client.subscription_url ?? ''

  function handleRotate() {
    rotate.mutate(undefined, {
      onSuccess: () => {
        toast({
          title: 'Configuration refreshed',
          description: 'A new subscription link was issued. The old link no longer works.',
        })
        setConfirmRotate(false)
      },
      onError: () => toast({ title: 'Could not refresh configuration', variant: 'destructive' }),
    })
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Subscription</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-6 md:grid-cols-[1fr_auto]">
            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="sub-url">Subscription URL</Label>
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
                  Add this link to a client app to receive all granted configs automatically.
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => setConfirmRotate(true)}>
                  <RefreshCw className="h-4 w-4" />
                  Refresh configuration
                </Button>
                <Button variant="outline" asChild>
                  <a href={`/api/clients/${client.id}/config`} download>
                    <Download className="h-4 w-4" />
                    Download config
                  </a>
                </Button>
              </div>
            </div>
            <div className="flex justify-center md:justify-end">
              {subUrl ? (
                <QrCode text={subUrl} size={200} />
              ) : (
                <div className="text-sm text-muted-foreground">No subscription URL</div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <div>
        <h3 className="mb-3 text-sm font-medium">Per-inbound links</h3>
        {linksLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        ) : !client.enabled ? (
          <EmptyState
            icon={Link2Off}
            title="Client is disabled"
            description="Enable the client to generate connection links from its granted inbounds."
          />
        ) : !links || links.length === 0 ? (
          <EmptyState
            icon={Link2Off}
            title="No active links"
            description="Links come only from enabled granted inbounds. Grant access on the Access tab (and make sure the inbounds are enabled)."
          />
        ) : (
          <Card className="divide-y p-0">
            {links.map((link) => (
              <LinkRow key={link.inbound_id} link={link} />
            ))}
          </Card>
        )}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Credentials</CardTitle>
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
              <Label className="text-xs text-muted-foreground">Password</Label>
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
            <DialogTitle>Refresh configuration?</DialogTitle>
            <DialogDescription>
              This issues a brand-new subscription link. The current link will immediately stop
              working, so the client will need the new one.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmRotate(false)} disabled={rotate.isPending}>
              Cancel
            </Button>
            <Button onClick={handleRotate} disabled={rotate.isPending}>
              {rotate.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Refresh
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function AccessTab({ client }: { client: Client }) {
  const { toast } = useToast()
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
      onSuccess: () => toast({ title: 'Access updated' }),
      onError: () => toast({ title: 'Could not update access', variant: 'destructive' }),
    })
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-muted-foreground">
          Choose which servers and protocols this client can use. Each selected inbound becomes a
          connection in its subscription.
        </p>
        <Button onClick={handleSave} disabled={!dirty || setInbounds.isPending}>
          {setInbounds.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
          Save access
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
          title="No servers available"
          description="Add a server with inbounds before granting access."
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

  const { data: client, isLoading } = useClient(id)
  const toggle = useToggleClient(id)
  const del = useDeleteClient()

  const [editOpen, setEditOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  function handleToggle() {
    if (!client) return
    toggle.mutate(!client.enabled, {
      onSuccess: () =>
        toast({ title: client.enabled ? 'Client disabled' : 'Client enabled' }),
      onError: () => toast({ title: 'Action failed', variant: 'destructive' }),
    })
  }

  function handleDelete() {
    del.mutate(id, {
      onSuccess: () => {
        toast({ title: 'Client deleted' })
        navigate('/clients')
      },
      onError: () => toast({ title: 'Could not delete client', variant: 'destructive' }),
    })
  }

  return (
    <div>
      <Link
        to="/clients"
        className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        Clients
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
              Edit
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleToggle}
              disabled={toggle.isPending}
            >
              <Power className="h-4 w-4" />
              {client.enabled ? 'Disable' : 'Enable'}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="text-destructive hover:text-destructive"
              onClick={() => setConfirmDelete(true)}
            >
              <Trash2 className="h-4 w-4" />
              Delete
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
              {client.enabled ? 'Enabled' : 'Disabled'}
            </Badge>
            <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
              <Users className="h-3.5 w-3.5" />
              {client.inbound_ids?.length ?? 0} granted inbounds
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
              <TabsTrigger value="connection">Connection</TabsTrigger>
              <TabsTrigger value="access">Access</TabsTrigger>
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
                <DialogTitle>Delete client?</DialogTitle>
                <DialogDescription>
                  “{client.name}” and its subscription link will be permanently removed. This
                  cannot be undone.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button
                  variant="ghost"
                  onClick={() => setConfirmDelete(false)}
                  disabled={del.isPending}
                >
                  Cancel
                </Button>
                <Button variant="destructive" onClick={handleDelete} disabled={del.isPending}>
                  {del.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
                  Delete
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </>
      )}
    </div>
  )
}
