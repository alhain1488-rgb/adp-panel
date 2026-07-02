import { useEffect, useRef, useState } from 'react'
import {
  Monitor,
  Moon,
  Sun,
  ShieldCheck,
  KeyRound,
  Loader2,
  Copy,
  Check,
  Download,
  Upload,
  DatabaseBackup,
  AlertTriangle,
} from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Separator } from '@/components/ui/separator'
import { useToast } from '@/components/ui/use-toast'
import { useSettings, useUpdateSettings } from '@/api/hooks'
import type { Settings, TotpSetup } from '@/api/types'
import { useTheme, type Theme } from '@/theme/theme-provider'
import { useAuth } from '@/auth/auth-context'
import { api, RequestError } from '@/api/client'
import { downloadBackup, uploadBackup, type ImportReport } from '@/api/backup'
import { cn } from '@/lib/utils'

export default function SettingsPage() {
  return (
    <div>
      <PageHeader title="Settings" description="Configure your panel and account" />

      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">General</TabsTrigger>
          <TabsTrigger value="appearance">Appearance</TabsTrigger>
          <TabsTrigger value="security">Security</TabsTrigger>
          <TabsTrigger value="backup">Backup</TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="mt-6">
          <GeneralTab />
        </TabsContent>
        <TabsContent value="appearance" className="mt-6">
          <AppearanceTab />
        </TabsContent>
        <TabsContent value="security" className="mt-6">
          <SecurityTab />
        </TabsContent>
        <TabsContent value="backup" className="mt-6">
          <BackupTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

// ---- General ----
function GeneralTab() {
  const { data, isLoading } = useSettings()
  const update = useUpdateSettings()
  const { toast } = useToast()

  const [form, setForm] = useState<Settings>({})

  useEffect(() => {
    if (data) setForm(data)
  }, [data])

  if (isLoading || !data) {
    return (
      <Card className="max-w-2xl">
        <CardHeader>
          <Skeleton className="h-6 w-40" />
          <Skeleton className="mt-2 h-4 w-64" />
        </CardHeader>
        <CardContent className="space-y-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="space-y-2">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-9 w-full" />
            </div>
          ))}
        </CardContent>
      </Card>
    )
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    update.mutate(form, {
      onSuccess: () => toast({ title: 'Settings saved' }),
      onError: (err) =>
        toast({
          variant: 'destructive',
          title: 'Failed to save',
          description: err instanceof RequestError ? err.message : 'Please try again.',
        }),
    })
  }

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>General</CardTitle>
        <CardDescription>Panel-wide options used to build subscriptions and sync.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="domain">Domain</Label>
            <Input
              id="domain"
              placeholder="panel.example.com"
              value={form.domain ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, domain: e.target.value }))}
            />
            <p className="text-xs text-muted-foreground">Public hostname clients connect to.</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="subscription_base_url">Subscription base URL</Label>
            <Input
              id="subscription_base_url"
              placeholder="https://panel.example.com/sub"
              value={form.subscription_base_url ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, subscription_base_url: e.target.value }))}
            />
            <p className="text-xs text-muted-foreground">Prefix used when generating client subscription links.</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="sync_interval_seconds">Sync interval (seconds)</Label>
            <Input
              id="sync_interval_seconds"
              type="number"
              min={0}
              value={form.sync_interval_seconds ?? ''}
              onChange={(e) =>
                setForm((f) => ({
                  ...f,
                  sync_interval_seconds: e.target.value === '' ? undefined : Number(e.target.value),
                }))
              }
            />
            <p className="text-xs text-muted-foreground">How often the panel pushes config to servers.</p>
          </div>

          <Separator />

          <div className="space-y-2">
            <Label>Hysteria engine</Label>
            <div className="flex items-center gap-2">
              <Badge variant="secondary" className="font-mono">
                {data.hysteria_engine ?? 'sing-box'}
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              Read-only. Chosen in the deployment docs and applied at deploy time.
            </p>
          </div>

          <div className="flex justify-end">
            <Button type="submit" disabled={update.isPending}>
              {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              Save
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

// ---- Appearance ----
const THEME_OPTIONS: { value: Theme; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]

function AppearanceTab() {
  const { theme, setTheme } = useTheme()

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
        <CardDescription>Choose how the panel looks. Saved locally on this device.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-3 gap-3">
          {THEME_OPTIONS.map((opt) => {
            const active = theme === opt.value
            const Icon = opt.icon
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => setTheme(opt.value)}
                aria-pressed={active}
                className={cn(
                  'flex flex-col items-center gap-2 rounded-lg border p-4 text-sm font-medium transition-colors',
                  active
                    ? 'border-primary bg-primary/5 text-foreground ring-1 ring-primary'
                    : 'border-input text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )}
              >
                <Icon className="h-5 w-5" />
                {opt.label}
              </button>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}

// ---- Security ----
function SecurityTab() {
  const { admin } = useAuth()
  const [open, setOpen] = useState(false)
  const enabled = admin?.totp_enabled ?? false

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>Two-factor authentication</CardTitle>
        <CardDescription>
          Protect your account with a time-based one-time password (TOTP).
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between rounded-lg border p-4">
          <div className="flex items-center gap-3">
            <div className="rounded-lg bg-primary/10 p-2.5 text-primary">
              <ShieldCheck className="h-5 w-5" />
            </div>
            <div>
              <p className="text-sm font-medium">Authenticator app</p>
              <p className="text-xs text-muted-foreground">
                {enabled ? 'Two-factor is currently active.' : 'Two-factor is not enabled.'}
              </p>
            </div>
          </div>
          <Badge variant={enabled ? 'success' : 'secondary'}>{enabled ? 'Enabled' : 'Disabled'}</Badge>
        </div>

        <Button variant="outline" onClick={() => setOpen(true)}>
          <KeyRound className="h-4 w-4" />
          {enabled ? 'Re-enroll 2FA' : 'Configure 2FA'}
        </Button>
      </CardContent>

      <TwoFactorDialog open={open} onOpenChange={setOpen} />
    </Card>
  )
}

// ---- Backup & Restore ----
function BackupTab() {
  const { toast } = useToast()

  // Export
  const [exportPass, setExportPass] = useState('')
  const [exporting, setExporting] = useState(false)

  async function handleExport() {
    if (exportPass.length < 8) return
    setExporting(true)
    try {
      await downloadBackup(exportPass)
      toast({
        title: 'Backup downloaded',
        description: 'Keep the file and its passphrase together, somewhere safe.',
      })
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Backup failed',
        description: err instanceof RequestError ? err.message : 'Please try again.',
      })
    } finally {
      setExporting(false)
    }
  }

  // Import
  const fileRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [importPass, setImportPass] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [importing, setImporting] = useState(false)
  const [done, setDone] = useState<ImportReport | null>(null)

  async function handleImport() {
    if (!file || importPass.length < 1) return
    setImporting(true)
    try {
      const report = await uploadBackup(file, importPass)
      setConfirmOpen(false)
      setDone(report)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: 'Restore failed',
        description: err instanceof RequestError ? err.message : 'Please try again.',
      })
    } finally {
      setImporting(false)
    }
  }

  return (
    <div className="grid max-w-2xl gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Download className="h-5 w-5 text-primary" />
            Export backup
          </CardTitle>
          <CardDescription>
            Download an encrypted snapshot of everything — servers, inbounds, clients and their
            grants. Restore it on any fresh install to migrate.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="export-pass">Encryption passphrase</Label>
            <Input
              id="export-pass"
              type="password"
              autoComplete="new-password"
              placeholder="At least 8 characters"
              value={exportPass}
              onChange={(e) => setExportPass(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              The backup is useless without this passphrase — you'll need it to restore. There is no
              way to recover it.
            </p>
          </div>
          <div className="flex justify-end">
            <Button onClick={handleExport} disabled={exporting || exportPass.length < 8}>
              {exporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
              Download backup
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Upload className="h-5 w-5 text-primary" />
            Restore from backup
          </CardTitle>
          <CardDescription>
            Import a backup file. This <span className="font-medium text-foreground">replaces all
            current data</span> and restarts the panel.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="import-file">Backup file</Label>
            <Input
              id="import-file"
              ref={fileRef}
              type="file"
              accept=".adpbak"
              className="cursor-pointer file:mr-3 file:cursor-pointer file:rounded file:border-0 file:bg-muted file:px-2 file:py-1 file:text-sm"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="import-pass">Passphrase</Label>
            <Input
              id="import-pass"
              type="password"
              autoComplete="off"
              placeholder="The passphrase this backup was made with"
              value={importPass}
              onChange={(e) => setImportPass(e.target.value)}
            />
          </div>
          <div className="flex justify-end">
            <Button
              variant="destructive"
              onClick={() => setConfirmOpen(true)}
              disabled={!file || importPass.length < 1}
            >
              <Upload className="h-4 w-4" />
              Restore
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Destructive-action confirmation */}
      <Dialog open={confirmOpen} onOpenChange={(v) => !importing && setConfirmOpen(v)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              Replace all data?
            </DialogTitle>
            <DialogDescription>
              Restoring overwrites every server, inbound and client currently in this panel, then
              restarts it. After it comes back, log in with the admin credentials from the{' '}
              <span className="font-medium text-foreground">backed-up</span> panel — not this one.
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setConfirmOpen(false)} disabled={importing}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleImport} disabled={importing}>
              {importing && <Loader2 className="h-4 w-4 animate-spin" />}
              Restore &amp; restart
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Success + restart notice */}
      <Dialog open={!!done} onOpenChange={(v) => !v && setDone(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <DatabaseBackup className="h-5 w-5 text-primary" />
              Restore applied
            </DialogTitle>
            <DialogDescription>
              Imported {done?.servers ?? 0} server{done?.servers === 1 ? '' : 's'},{' '}
              {done?.inbounds ?? 0} inbound{done?.inbounds === 1 ? '' : 's'} and {done?.clients ?? 0}{' '}
              client{done?.clients === 1 ? '' : 's'}. The panel is restarting — give it ~15 seconds,
              then reload and sign in with the backed-up admin credentials.
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end">
            <Button onClick={() => window.location.reload()}>Reload panel</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function TwoFactorDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const { toast } = useToast()
  const { refresh } = useAuth()

  const [setup, setSetup] = useState<TotpSetup | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!open) {
      setSetup(null)
      setError(null)
      setCode('')
      setCopied(false)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .post<TotpSetup>('/api/auth/2fa/setup')
      .then((res) => {
        if (!cancelled) setSetup(res)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof RequestError ? err.message : 'Failed to start enrollment')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open])

  async function copySecret() {
    if (!setup?.secret) return
    await navigator.clipboard.writeText(setup.secret)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  async function handleEnable(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await api.post('/api/auth/2fa/enable', { code })
      toast({ title: 'Two-factor enabled' })
      await refresh()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof RequestError ? err.message : 'Invalid code. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Configure two-factor authentication</DialogTitle>
          <DialogDescription>
            Scan the QR code with your authenticator app, then enter the 6-digit code to confirm.
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="space-y-4">
            <Skeleton className="mx-auto h-44 w-44" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : setup ? (
          <form onSubmit={handleEnable} className="space-y-4">
            <div className="flex justify-center">
              <img
                src={setup.qr}
                alt="Two-factor enrollment QR code"
                className="h-44 w-44 rounded-lg border bg-white p-2"
              />
            </div>

            {setup.secret && (
              <div className="space-y-1.5">
                <Label>Secret key</Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 truncate rounded-md border bg-muted px-3 py-2 font-mono text-xs">
                    {setup.secret}
                  </code>
                  <Button type="button" variant="outline" size="icon" onClick={copySecret}>
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    <span className="sr-only">Copy secret</span>
                  </Button>
                </div>
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="totp-code">Authentication code</Label>
              <Input
                id="totp-code"
                inputMode="numeric"
                autoFocus
                placeholder="123456"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                className="text-center text-lg tracking-[0.5em]"
              />
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}

            <Button type="submit" className="w-full" disabled={submitting || code.length !== 6}>
              {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
              Enable
            </Button>
          </form>
        ) : (
          <p className="text-sm text-destructive">{error ?? 'Something went wrong.'}</p>
        )}
      </DialogContent>
    </Dialog>
  )
}
