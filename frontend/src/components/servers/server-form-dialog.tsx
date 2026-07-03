import { useEffect, useState } from 'react'
import { Loader2, AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
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
import { useT } from '@/i18n/i18n'
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
  const t = useT()
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
  const [hyConfigPath, setHyConfigPath] = useState('/etc/sing-box/config.json')
  const [hyServiceName, setHyServiceName] = useState('sing-box')
  const [wipeExisting, setWipeExisting] = useState(false)

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
    setHyConfigPath(server?.hysteria_config_path ?? '/etc/sing-box/config.json')
    setHyServiceName(server?.hysteria_service_name ?? 'sing-box')
    setWipeExisting(false)
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
      hysteria_config_path: hyConfigPath.trim() || '/etc/sing-box/config.json',
      hysteria_service_name: hyServiceName.trim() || 'sing-box',
      wipe_existing: !isEdit && wipeExisting,
    }

    try {
      if (isEdit && server) {
        await updateServer.mutateAsync(input)
        toast({ title: t('serverForm.updated') })
      } else {
        await createServer.mutateAsync(input)
        toast({ title: t('serverForm.added') })
      }
      onOpenChange(false)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: isEdit ? t('serverForm.updateFailed') : t('serverForm.addFailed'),
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isEdit ? t('serverForm.editTitle') : t('serverForm.addTitle')}</DialogTitle>
            <DialogDescription>
              {isEdit ? t('serverForm.editDesc') : t('serverForm.addDesc')}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Label htmlFor="sv-name">{t('serverForm.name')}</Label>
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
              <Label htmlFor="sv-host">{t('serverForm.host')}</Label>
              <Input
                id="sv-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder={t('serverForm.hostPlaceholder')}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sv-port">{t('serverForm.sshPort')}</Label>
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
              <Label htmlFor="sv-user">{t('serverForm.sshUser')}</Label>
              <Input id="sv-user" value={sshUser} onChange={(e) => setSshUser(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="sv-auth">{t('serverForm.authMethod')}</Label>
              <select
                id="sv-auth"
                className={selectClass}
                value={authMethod}
                onChange={(e) => setAuthMethod(e.target.value as AuthMethod)}
              >
                <option value="key">{t('serverForm.privateKey')}</option>
                <option value="password">{t('serverForm.password')}</option>
              </select>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="sv-secret">{authMethod === 'key' ? t('serverForm.privateKey') : t('serverForm.password')}</Label>
            {authMethod === 'key' ? (
              <Textarea
                id="sv-secret"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder={isEdit ? t('serverForm.keyPlaceholderEdit') : '-----BEGIN OPENSSH PRIVATE KEY-----'}
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
                placeholder={isEdit ? t('serverForm.passwordPlaceholderEdit') : t('serverForm.passwordPlaceholder')}
                autoComplete="new-password"
              />
            )}
            {isEdit && (
              <p className="text-xs text-muted-foreground">{t('serverForm.keepSecretHint')}</p>
            )}
          </div>

          {authMethod === 'key' && (
            <div className="space-y-2">
              <Label htmlFor="sv-passphrase">{t('serverForm.passphrase')}</Label>
              <Input
                id="sv-passphrase"
                type="password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder={t('serverForm.passphrasePlaceholder')}
                autoComplete="new-password"
              />
            </div>
          )}

          <div className="space-y-3 rounded-md border p-3">
            <div>
              <p className="text-sm font-medium">{t('serverForm.enginePaths')}</p>
              <p className="text-xs text-muted-foreground">
                {t('serverForm.enginePathsHint')}
              </p>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="sv-config">{t('serverForm.xrayConfigPath')}</Label>
                <Input
                  id="sv-config"
                  value={configPath}
                  onChange={(e) => setConfigPath(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="sv-service">{t('serverForm.xrayServiceName')}</Label>
                <Input
                  id="sv-service"
                  value={serviceName}
                  onChange={(e) => setServiceName(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="sv-hyconfig">{t('serverForm.hyConfigPath')}</Label>
                <Input
                  id="sv-hyconfig"
                  value={hyConfigPath}
                  onChange={(e) => setHyConfigPath(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="sv-hyservice">{t('serverForm.hyServiceName')}</Label>
                <Input
                  id="sv-hyservice"
                  value={hyServiceName}
                  onChange={(e) => setHyServiceName(e.target.value)}
                  className="font-mono text-xs"
                />
              </div>
            </div>
          </div>

          {!isEdit && (
            <div className="space-y-2 rounded-md border border-destructive/40 bg-destructive/5 p-3">
              <label htmlFor="sv-wipe" className="flex cursor-pointer items-start gap-3">
                <Checkbox
                  id="sv-wipe"
                  checked={wipeExisting}
                  onCheckedChange={(v) => setWipeExisting(v === true)}
                  className="mt-0.5"
                />
                <span className="space-y-1">
                  <span className="flex items-center gap-1.5 text-sm font-medium">
                    <AlertTriangle className="h-4 w-4 text-destructive" />
                    {t('serverForm.wipeTitle')}
                  </span>
                  <span className="block text-xs text-muted-foreground">
                    {t('serverForm.wipeDesc')}
                    <span className="font-medium text-foreground"> {t('serverForm.wipeIrreversible')}</span>
                  </span>
                </span>
              </label>
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending && <Loader2 className="h-4 w-4 animate-spin" />}
              {isEdit ? t('serverForm.saveChanges') : t('serverForm.addTitle')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
