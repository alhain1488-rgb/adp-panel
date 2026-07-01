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
} from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState } from '@/components/common/misc'
import { CopyButton } from '@/components/common/copy-button'
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
import { cn, formatRelativeTime } from '@/lib/utils'
import {
  useClients,
  useCreateClient,
  useDeleteClient,
  useRotateToken,
  useToggleClient,
} from '@/api/hooks'
import type { Client } from '@/api/types'

function EnabledDot({ enabled }: { enabled: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-sm">
      <span
        className={cn('h-1.5 w-1.5 rounded-full', enabled ? 'bg-success' : 'bg-muted-foreground')}
      />
      <span className={enabled ? '' : 'text-muted-foreground'}>
        {enabled ? 'Enabled' : 'Disabled'}
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
      toast({ title: 'Client created', description: `“${client.name}” is ready to configure.` })
      onOpenChange(false)
      reset()
      navigate(`/clients/${client.id}`)
    } catch {
      toast({ title: 'Could not create client', variant: 'destructive' })
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
            <DialogTitle>New client</DialogTitle>
            <DialogDescription>
              Just give it a name — you can grant server access and share the subscription
              afterwards.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2 py-4">
            <Label htmlFor="client-name">Name</Label>
            <Input
              id="client-name"
              autoFocus
              value={name}
              placeholder="e.g. Alice's phone"
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
              Cancel
            </Button>
            <Button type="submit" disabled={createClient.isPending || !name.trim()}>
              {createClient.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Create client
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ClientRow({ client }: { client: Client }) {
  const { toast } = useToast()
  const toggle = useToggleClient(client.id)
  const rotate = useRotateToken(client.id)
  const del = useDeleteClient()
  const [confirmDelete, setConfirmDelete] = useState(false)

  const grantCount = client.inbound_ids?.length ?? 0

  function handleToggle() {
    toggle.mutate(!client.enabled, {
      onSuccess: () =>
        toast({ title: client.enabled ? 'Client disabled' : 'Client enabled' }),
      onError: () => toast({ title: 'Action failed', variant: 'destructive' }),
    })
  }

  function handleRotate() {
    rotate.mutate(undefined, {
      onSuccess: () =>
        toast({
          title: 'Subscription token rotated',
          description: 'The previous subscription link no longer works.',
        }),
      onError: () => toast({ title: 'Could not rotate token', variant: 'destructive' }),
    })
  }

  function handleDelete() {
    del.mutate(client.id, {
      onSuccess: () => {
        toast({ title: 'Client deleted' })
        setConfirmDelete(false)
      },
      onError: () => toast({ title: 'Could not delete client', variant: 'destructive' }),
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
          {grantCount} {grantCount === 1 ? 'inbound' : 'inbounds'}
        </TableCell>
        <TableCell className="max-w-[16rem] truncate text-muted-foreground">
          {client.remark || '—'}
        </TableCell>
        <TableCell className="whitespace-nowrap text-muted-foreground">
          {formatRelativeTime(client.created_at)}
        </TableCell>
        <TableCell>
          {client.subscription_url ? (
            <CopyButton value={client.subscription_url} size="sm" label="Subscription" />
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
                  Open
                </Link>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleToggle} disabled={toggle.isPending}>
                <Power className="h-4 w-4" />
                {client.enabled ? 'Disable' : 'Enable'}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleRotate} disabled={rotate.isPending}>
                <RefreshCw className="h-4 w-4" />
                Rotate token
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 className="h-4 w-4" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Delete client?</DialogTitle>
            <DialogDescription>
              “{client.name}” and its subscription link will be permanently removed. This cannot be
              undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmDelete(false)} disabled={del.isPending}>
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
  )
}

export default function ClientsPage() {
  const { data: clients, isLoading } = useClients()
  const [createOpen, setCreateOpen] = useState(false)

  return (
    <div>
      <PageHeader title="Clients" description="People and devices that connect through your servers">
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" />
          Add client
        </Button>
      </PageHeader>

      {isLoading || !clients ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-14" />
          ))}
        </div>
      ) : clients.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No clients yet"
          description="Create a client to generate a subscription link and grant it access to your servers."
          action={
            <Button onClick={() => setCreateOpen(true)}>
              <Plus className="h-4 w-4" />
              Add client
            </Button>
          }
        />
      ) : (
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>State</TableHead>
                <TableHead>Access</TableHead>
                <TableHead>Remark</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Subscription</TableHead>
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
