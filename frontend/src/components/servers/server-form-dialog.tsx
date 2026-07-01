import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useToast } from '@/components/ui/use-toast'
import { useCreateServer, useUpdateServer } from '@/api/hooks'
import type { Server, ServerInput } from '@/api/types'

type AuthMethod = 'key' | 'password'

const selectClass =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50'

export function ServerFormDialog({
  open,
  onOpenChange,
  server,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  server?: Server
}) {
  const isEdit = !!server
  const { toast } = useToast()
  const createServer = useCreateServer()
  const updateServer = useUpdateServer(server?.id ?? 0)

  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [sshPort, setSshPort] = useState('22')
  const [sshUser, setSshUser] = useState('root')
  const [authMethod, setAuthMethod] = useState<AuthMethod>('key')
  const [secret, setSecret] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [configPath, setConfigPath] = useState('/usr/local/etc/xray/config.json')
  const [serviceName, setServiceName] = useState('xray')

  useEffect(() => {
    if (!open) return
    setName(server?.name ?? '')
    setHost(server?.host ?? '')
    setSshPort(server?.ssh_port != null ? String(server.ssh_port) : '22')
    setSshUser(server?.ssh_user ?? 'root')
    setAuthMethod((server?.ssh_auth_method as AuthMethod) ?? 'key')
    setSecret('')
    setPassphrase('')
    setConfigPath(server?.xray_config_path ?? '/usr/local/etc/xray/config.json')
    setServiceName(server?.xray_service_name ?? 'xray')
  }, [open, server])

  const pending = createServer.isPending || updateServer.isPending

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const input: ServerInput = {
      name: name.trim(),
      host: host.trim(),
      ssh_port: Number(sshPort) || 22,
      ssh_user: sshUser.trim() || 'root',
      ssh_auth_method: authMethod,
      ssh_secret: secret.trim() || undefined,
      ssh_passphrase: passphrase.trim() || undefined,
      xray_config_path: configPath.trim() || '/usr/local/etc/xray/config.json',
      xray_service_name: serviceName.trim() || 'xray',
    }

    try {
      if (isEdit && server) {
        await updateServer.mutateAsync(input)
        toast({ title: 'Server updated' })
      } else {
        await createServer.mutateAsync(input)
        toast({ title: 'Server added' })
      }
      onOpenChange(false)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: isEdit ? 'Failed to update server' : 'Failed to add server',
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isEdit ? 'Edit server' : 'Add server'}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? 'Update the connection details for this server.'
                : 'Connect a new server over SSH to manage its inbounds.'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Label htmlFor="sv-name">Name</Label>
            <Input
              id="sv-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Frankfurt-01"
              required
            />
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div className="col-span-2 space-y-2">
              <Label htmlFor="sv-host">Host</Label>
              <Input
                id="sv-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="1.2.3.4 or host.example.com"
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sv-port">SSH port</Label>
              <Input
                id="sv-port"
                type="number"
                min={1}
                max={65535}
                value={sshPort}
                onChange={(e) => setSshPort(e.target.value)}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="sv-user">SSH user</Label>
              <Input id="sv-user" value={sshUser} onChange={(e) => setSshUser(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sv-auth">Auth method</Label>
              <select
                id="sv-auth"
                className={selectClass}
                value={authMethod}
                onChange={(e) => setAuthMethod(e.target.value as AuthMethod)}
              >
                <option value="key">Private key</option>
                <option value="password">Password</option>
              </select>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="sv-secret">{authMethod === 'key' ? 'Private key' : 'Password'}</Label>
            {authMethod === 'key' ? (
              <Textarea
                id="sv-secret"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder={isEdit ? 'Leave blank to keep existing key' : '-----BEGIN OPENSSH PRIVATE KEY-----'}
                rows={5}
                className="font-mono text-xs"
                spellCheck={false}
              />
            ) : (
              <Input
                id="sv-secret"
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder={isEdit ? 'Leave blank to keep existing password' : 'SSH password'}
                autoComplete="new-password"
              />
            )}
            {isEdit && (
              <p className="text-xs text-muted-foreground">Leave blank to keep the current secret.</p>
            )}
          </div>

          {authMethod === 'key' && (
            <div className="space-y-2">
              <Label htmlFor="sv-passphrase">Key passphrase (optional)</Label>
              <Input
                id="sv-passphrase"
                type="password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder="Only if the key is encrypted"
                autoComplete="new-password"
              />
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="sv-config">Xray config path</Label>
              <Input
                id="sv-config"
                value={configPath}
                onChange={(e) => setConfigPath(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sv-service">Xray service name</Label>
              <Input
                id="sv-service"
                value={serviceName}
                onChange={(e) => setServiceName(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {pending && <Loader2 className="h-4 w-4 animate-spin" />}
              {isEdit ? 'Save changes' : 'Add server'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
