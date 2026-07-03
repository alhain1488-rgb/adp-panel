import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  Users,
  Plus,
  MoreHorizontal,
  ExternalLink,
  Power,
  RefreshCw,
  Trash2,
  Loader2,
  ListFilter,
} from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState } from '@/components/common/misc'
import { CopyButton } from '@/components/common/copy-button'
import { QrCode } from '@/components/common/qr-code'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useToast } from '@/components/ui/use-toast'
import { cn } from '@/lib/utils'
import { useT, useRelTime } from '@/i18n/i18n'
import {
  useClients,
  useCreateClient,
  useDeleteClient,
  useRotateToken,
  useToggleClient,
} from '@/api/hooks'
import type { Client } from '@/api/types'

function EnabledDot({ enabled }: { enabled: boolean }) {
  const t = useT()
  return (
    <span className="inline-flex items-center gap-1.5 text-sm">
      <span
        className={cn('h-1.5 w-1.5 rounded-full', enabled ? 'bg-success' : 'bg-muted-foreground')}
      />
      <span className={enabled ? '' : 'text-muted-foreground'}>
        {enabled ? t('common.enabled') : t('common.disabled')}
      </span>
    </span>
  )
}

function CreateClientDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const navigate = useNavigate()
  const { toast } = useToast()
  const t = useT()
  const createClient = useCreateClient()
  const [name, setName] = useState('')

  function reset() {
    setName('')
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return
    try {
      const client = await createClient.mutateAsync({ name: trimmed })
      toast({ title: t('clients.created.title'), description: t('clients.created.desc', { name: client.name }) })
      onOpenChange(false)
      reset()
      navigate(`/clients/${client.id}`)
    } catch {
      toast({ title: t('clients.createFailed'), variant: 'destructive' })
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        onOpenChange(o)
        if (!o) reset()
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t('clients.new.title')}</DialogTitle>
            <DialogDescription>{t('clients.new.desc')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2 py-4">
            <Label htmlFor="client-name">{t('clients.col.name')}</Label>
            <Input
              id="client-name"
              autoFocus
              value={name}
              placeholder={t('clients.new.namePlaceholder')}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={createClient.isPending}
            >
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={createClient.isPending || !name.trim()}>
              {createClient.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('clients.add')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ClientRow({ client }: { client: Client }) {
  const { toast } = useToast()
  const t = useT()
  const rel = useRelTime()
  const toggle = useToggleClient(client.id)
  const rotate = useRotateToken(client.id)
  const del = useDeleteClient()
  const [confirmDelete, setConfirmDelete] = useState(false)

  const grantCount = client.inbound_ids?.length ?? 0

  function handleToggle() {
    toggle.mutate(!client.enabled, {
      onSuccess: () =>
        toast({ title: client.enabled ? t('clients.toggle.disabled') : t('clients.toggle.enabled') }),
      onError: () => toast({ title: t('clients.actionFailed'), variant: 'destructive' }),
    })
  }

  function handleRotate() {
    rotate.mutate(undefined, {
      onSuccess: () =>
        toast({
          title: t('clients.rotated.title'),
          description: t('clients.rotated.desc'),
        }),
      onError: () => toast({ title: t('clients.rotateFailed'), variant: 'destructive' }),
    })
  }

  function handleDelete() {
    del.mutate(client.id, {
      onSuccess: () => {
        toast({ title: t('clients.deleted') })
        setConfirmDelete(false)
      },
      onError: () => toast({ title: t('clients.deleteFailed'), variant: 'destructive' }),
    })
  }

  return (
    <>
      <TableRow>
        <TableCell>
          <Link to={`/clients/${client.id}`} className="font-medium hover:underline">
            {client.name}
          </Link>
        </TableCell>
        <TableCell>
          <EnabledDot enabled={client.enabled} />
        </TableCell>
        <TableCell className="tabular-nums">
          {grantCount === 1 ? t('clients.inbounds.one', { n: grantCount }) : t('clients.inbounds', { n: grantCount })}
        </TableCell>
        <TableCell className="max-w-[16rem] truncate text-muted-foreground">
          {client.remark || '—'}
        </TableCell>
        <TableCell className="whitespace-nowrap text-muted-foreground">
          {rel(client.created_at)}
        </TableCell>
        <TableCell>
          {client.subscription_url ? (
            <CopyButton value={client.subscription_url} size="sm" label={t('clients.col.subscription')} />
          ) : (
            <span className="text-xs text-muted-foreground">—</span>
          )}
        </TableCell>
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label="Actions">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem asChild>
                <Link to={`/clients/${client.id}`}>
                  <ExternalLink className="h-4 w-4" />
                  {t('common.open')}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleToggle} disabled={toggle.isPending}>
                <Power className="h-4 w-4" />
                {client.enabled ? t('common.disable') : t('common.enable')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleRotate} disabled={rotate.isPending}>
                <RefreshCw className="h-4 w-4" />
                {t('clients.rotate')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 className="h-4 w-4" />
                {t('common.delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('clients.delete.title')}</DialogTitle>
            <DialogDescription>{t('clients.delete.desc', { name: client.name })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmDelete(false)} disabled={del.isPending}>
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
  )
}

// A shared, static list of RU services that should route directly (bypass the
// VPN). Served as a plain file by the panel; users plug the URL into their
// client's direct/bypass routing, or scan the QR to open it.
// The same curated list, served in the format each client understands. The
// derived files live in /public and are generated from ru-direct-domains.txt.
const DIRECT_FORMATS = [
  { id: 'singbox', file: 'ru-direct.singbox.json', labelKey: 'directDomains.fmt.singbox', hintKey: 'directDomains.hint.singbox' },
  { id: 'clash', file: 'ru-direct.clash.yaml', labelKey: 'directDomains.fmt.clash', hintKey: 'directDomains.hint.clash' },
  { id: 'txt', file: 'ru-direct-domains.txt', labelKey: 'directDomains.fmt.txt', hintKey: 'directDomains.hint.txt' },
] as const

type DirectFormatId = (typeof DIRECT_FORMATS)[number]['id']

function DirectDomainsCard() {
  const t = useT()
  const [fmtId, setFmtId] = useState<DirectFormatId>('singbox')
  const fmt = DIRECT_FORMATS.find((f) => f.id === fmtId) ?? DIRECT_FORMATS[0]
  const url = `${window.location.origin}/${fmt.file}`
  return (
    <Card className="mb-6 p-5">
      <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex items-center gap-2">
            <ListFilter className="h-5 w-5 text-primary" />
            <h3 className="font-semibold">{t('directDomains.title')}</h3>
          </div>
          <p className="max-w-xl text-sm text-muted-foreground">{t('directDomains.desc')}</p>

          <div className="space-y-1.5">
            <label htmlFor="dd-format" className="text-xs font-medium text-muted-foreground">
              {t('directDomains.clientLabel')}
            </label>
            <select
              id="dd-format"
              value={fmtId}
              onChange={(e) => setFmtId(e.target.value as DirectFormatId)}
              className="flex h-9 w-full max-w-xs cursor-pointer rounded-md border border-input bg-background px-3 py-1 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {DIRECT_FORMATS.map((f) => (
                <option key={f.id} value={f.id}>
                  {t(f.labelKey)}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <Input
              readOnly
              value={url}
              onFocus={(e) => e.currentTarget.select()}
              className="max-w-md font-mono text-xs"
            />
            <CopyButton value={url} size="sm" label="URL" />
          </div>
          <p className="max-w-xl text-xs text-muted-foreground">{t(fmt.hintKey)}</p>
        </div>
        <div className="shrink-0 self-center">
          <QrCode text={url} size={132} />
        </div>
      </div>
    </Card>
  )
}

export default function ClientsPage() {
  const { data: clients, isLoading } = useClients()
  const [createOpen, setCreateOpen] = useState(false)
  const t = useT()

  return (
    <div>
      <PageHeader title={t('clients.title')} description={t('clients.subtitle')}>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" />
          {t('clients.add')}
        </Button>
      </PageHeader>

      <DirectDomainsCard />

      {isLoading || !clients ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : clients.length === 0 ? (
        <EmptyState
          icon={Users}
          title={t('clients.empty.title')}
          description={t('clients.empty.desc')}
          action={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" />
              {t('clients.add')}
            </Button>
          }
        />
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('clients.col.name')}</TableHead>
                <TableHead>{t('clients.col.state')}</TableHead>
                <TableHead>{t('clients.col.access')}</TableHead>
                <TableHead>{t('clients.col.remark')}</TableHead>
                <TableHead>{t('clients.col.created')}</TableHead>
                <TableHead>{t('clients.col.subscription')}</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {clients.map((client) => (
                <ClientRow key={client.id} client={client} />
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <CreateClientDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}
