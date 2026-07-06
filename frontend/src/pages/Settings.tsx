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
  Send,
  Mail,
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
import { Switch } from '@/components/ui/switch'
import { useToast } from '@/components/ui/use-toast'
import { useSettings, useUpdateSettings } from '@/api/hooks'
import type { Settings, TotpSetup } from '@/api/types'
import { useTheme, type Theme } from '@/theme/theme-provider'
import { useAuth } from '@/auth/auth-context'
import { api, RequestError } from '@/api/client'
import {
  downloadBackup,
  uploadBackup,
  useTelegramBackup,
  useUpdateTelegramBackup,
  useRunTelegramBackup,
  useMailConfig,
  useUpdateMailConfig,
  useTestMail,
  useEmailBackup,
  useUpdateEmailBackup,
  useRunEmailBackup,
  type ImportReport,
  type TelegramInput,
  type MailInput,
  type EmailBackupInput,
} from '@/api/backup'
import { cn } from '@/lib/utils'
import { useT } from '@/i18n/i18n'

export default function SettingsPage() {
  const t = useT()
  return (
    <div>
      <PageHeader title={t('settings.title')} description={t('settings.subtitle')} />

      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">{t('settings.tab.general')}</TabsTrigger>
          <TabsTrigger value="appearance">{t('settings.tab.appearance')}</TabsTrigger>
          <TabsTrigger value="security">{t('settings.tab.security')}</TabsTrigger>
          <TabsTrigger value="backup">{t('settings.tab.backup')}</TabsTrigger>
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
  const t = useT()

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
      onSuccess: () => toast({ title: t('settings.general.saved') }),
      onError: (err) =>
        toast({
          variant: 'destructive',
          title: t('settings.saveFailed'),
          description: err instanceof RequestError ? err.message : t('common.tryAgain'),
        }),
    })
  }

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>{t('settings.tab.general')}</CardTitle>
        <CardDescription>{t('settings.general.desc')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="space-y-2">
            <Label htmlFor="domain">{t('settings.general.domain')}</Label>
            <Input
              id="domain"
              placeholder="panel.example.com"
              value={form.domain ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, domain: e.target.value }))}
            />
            <p className="text-xs text-muted-foreground">{t('settings.general.domainHint')}</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="subscription_base_url">{t('settings.general.subBaseUrl')}</Label>
            <Input
              id="subscription_base_url"
              placeholder="https://panel.example.com/sub"
              value={form.subscription_base_url ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, subscription_base_url: e.target.value }))}
            />
            <p className="text-xs text-muted-foreground">{t('settings.general.subBaseUrlHint')}</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="sync_interval_seconds">{t('settings.general.syncInterval')}</Label>
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
            <p className="text-xs text-muted-foreground">{t('settings.general.syncIntervalHint')}</p>
          </div>

          <Separator />

          <div className="space-y-2">
            <Label>{t('settings.general.hysteriaEngine')}</Label>
            <div className="flex items-center gap-2">
              <Badge variant="secondary" className="font-mono">
                {data.hysteria_engine ?? 'sing-box'}
              </Badge>
            </div>
            <p className="text-xs text-muted-foreground">
              {t('settings.general.hysteriaEngineHint')}
            </p>
          </div>

          <div className="flex justify-end">
            <Button type="submit" disabled={update.isPending}>
              {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('common.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

// ---- Appearance ----
const THEME_OPTIONS: { value: Theme; labelKey: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { value: 'light', labelKey: 'settings.appearance.light', icon: Sun },
  { value: 'dark', labelKey: 'settings.appearance.dark', icon: Moon },
  { value: 'system', labelKey: 'settings.appearance.system', icon: Monitor },
]

function AppearanceTab() {
  const { theme, setTheme } = useTheme()
  const t = useT()

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>{t('settings.tab.appearance')}</CardTitle>
        <CardDescription>{t('settings.appearance.desc')}</CardDescription>
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
                {t(opt.labelKey)}
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
  const t = useT()

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>{t('settings.security.title')}</CardTitle>
        <CardDescription>
          {t('settings.security.desc')}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between rounded-lg border p-4">
          <div className="flex items-center gap-3">
            <div className="rounded-lg bg-primary/10 p-2.5 text-primary">
              <ShieldCheck className="h-5 w-5" />
            </div>
            <div>
              <p className="text-sm font-medium">{t('settings.security.authApp')}</p>
              <p className="text-xs text-muted-foreground">
                {enabled ? t('settings.security.active') : t('settings.security.inactive')}
              </p>
            </div>
          </div>
          <Badge variant={enabled ? 'success' : 'secondary'}>{enabled ? t('common.enabled') : t('common.disabled')}</Badge>
        </div>

        <Button variant="outline" onClick={() => setOpen(true)}>
          <KeyRound className="h-4 w-4" />
          {enabled ? t('settings.security.reenroll') : t('settings.security.configure')}
        </Button>
      </CardContent>

      <TwoFactorDialog open={open} onOpenChange={setOpen} />
    </Card>
  )
}

// ---- Backup & Restore ----
function BackupTab() {
  const { toast } = useToast()
  const t = useT()

  // Export
  const [exportPass, setExportPass] = useState('')
  const [exporting, setExporting] = useState(false)

  async function handleExport() {
    if (exportPass.length < 8) return
    setExporting(true)
    try {
      await downloadBackup(exportPass)
      toast({
        title: t('settings.backup.exportDone.title'),
        description: t('settings.backup.exportDone.desc'),
      })
    } catch (err) {
      toast({
        variant: 'destructive',
        title: t('settings.backup.exportFailed'),
        description: err instanceof RequestError ? err.message : t('common.tryAgain'),
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
        title: t('settings.backup.restoreFailed'),
        description: err instanceof RequestError ? err.message : t('common.tryAgain'),
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
            {t('settings.backup.export.title')}
          </CardTitle>
          <CardDescription>
            {t('settings.backup.export.desc')}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="export-pass">{t('settings.backup.encPassphrase')}</Label>
            <Input
              id="export-pass"
              type="password"
              autoComplete="new-password"
              placeholder={t('settings.backup.min8Placeholder')}
              value={exportPass}
              onChange={(e) => setExportPass(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              {t('settings.backup.encPassphraseHint')}
            </p>
          </div>
          <div className="flex justify-end">
            <Button onClick={handleExport} disabled={exporting || exportPass.length < 8}>
              {exporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
              {t('settings.backup.download')}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Upload className="h-5 w-5 text-primary" />
            {t('settings.backup.restore.title')}
          </CardTitle>
          <CardDescription>
            {t('settings.backup.restore.descPre')}{' '}
            <span className="font-medium text-foreground">{t('settings.backup.restore.descEmphasis')}</span>
            {t('settings.backup.restore.descPost')}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="import-file">{t('settings.backup.file')}</Label>
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
            <Label htmlFor="import-pass">{t('settings.backup.passphrase')}</Label>
            <Input
              id="import-pass"
              type="password"
              autoComplete="off"
              placeholder={t('settings.backup.importPassPlaceholder')}
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
              {t('settings.backup.restore.button')}
            </Button>
          </div>
        </CardContent>
      </Card>

      <TelegramBackupCard />
      <SmtpCard />
      <EmailBackupCard />

      {/* Destructive-action confirmation */}
      <Dialog open={confirmOpen} onOpenChange={(v) => !importing && setConfirmOpen(v)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              {t('settings.backup.confirm.title')}
            </DialogTitle>
            <DialogDescription>
              {t('settings.backup.confirm.descPre')}{' '}
              <span className="font-medium text-foreground">{t('settings.backup.confirm.descEmphasis')}</span>
              {t('settings.backup.confirm.descPost')}
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2">
            <Button variant="outline" onClick={() => setConfirmOpen(false)} disabled={importing}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={handleImport} disabled={importing}>
              {importing && <Loader2 className="h-4 w-4 animate-spin" />}
              {t('settings.backup.confirm.button')}
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
              {t('settings.backup.applied.title')}
            </DialogTitle>
            <DialogDescription>
              {t('settings.backup.applied.desc', {
                servers: done?.servers ?? 0,
                inbounds: done?.inbounds ?? 0,
                clients: done?.clients ?? 0,
              })}
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end">
            <Button onClick={() => window.location.reload()}>{t('settings.backup.reload')}</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function TelegramBackupCard() {
  const { data, isLoading } = useTelegramBackup()
  const update = useUpdateTelegramBackup()
  const runNow = useRunTelegramBackup()
  const { toast } = useToast()
  const t = useT()

  const [form, setForm] = useState<TelegramInput>({
    enabled: false,
    token: '',
    chat_id: '',
    passphrase: '',
    interval_hours: 24,
  })

  useEffect(() => {
    if (data) {
      setForm({
        enabled: data.enabled,
        token: '',
        chat_id: data.chat_id,
        passphrase: '',
        interval_hours: data.interval_hours,
      })
    }
  }, [data])

  const configured = !!data?.has_token && !!data?.has_passphrase && !!data?.chat_id

  function save() {
    update.mutate(form, {
      onSuccess: () => {
        toast({ title: t('settings.telegram.saved') })
        setForm((f) => ({ ...f, token: '', passphrase: '' }))
      },
      onError: (err) =>
        toast({
          variant: 'destructive',
          title: t('settings.saveFailed'),
          description: err instanceof RequestError ? err.message : t('common.tryAgain'),
        }),
    })
  }

  function sendNow() {
    runNow.mutate(undefined, {
      onSuccess: () => toast({ title: t('settings.telegram.sent') }),
      onError: (err) =>
        toast({
          variant: 'destructive',
          title: t('settings.telegram.sendFailed'),
          description: err instanceof RequestError ? err.message : t('settings.telegram.sendFailedDesc'),
        }),
    })
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-56" />
          <Skeleton className="mt-2 h-4 w-72" />
        </CardHeader>
        <CardContent className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Send className="h-5 w-5 text-primary" />
          {t('settings.telegram.title')}
        </CardTitle>
        <CardDescription>
          {t('settings.telegram.descPre')}{' '}
          <span className="font-mono">@BotFather</span>{t('settings.telegram.descPost')}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <p className="text-sm font-medium">{t('settings.telegram.scheduled')}</p>
            <p className="text-xs text-muted-foreground">{t('settings.telegram.scheduledHint')}</p>
          </div>
          <Switch
            checked={form.enabled}
            onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="tg-token">{t('settings.telegram.botToken')}</Label>
          <Input
            id="tg-token"
            type="password"
            autoComplete="off"
            placeholder={data?.has_token ? t('settings.telegram.storedPlaceholder') : '123456:ABC-DEF…'}
            value={form.token}
            onChange={(e) => setForm((f) => ({ ...f, token: e.target.value }))}
          />
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="tg-chat">{t('settings.telegram.chatId')}</Label>
            <Input
              id="tg-chat"
              placeholder="123456789"
              value={form.chat_id}
              onChange={(e) => setForm((f) => ({ ...f, chat_id: e.target.value }))}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="tg-interval">{t('settings.telegram.interval')}</Label>
            <Input
              id="tg-interval"
              type="number"
              min={1}
              value={form.interval_hours}
              onChange={(e) =>
                setForm((f) => ({ ...f, interval_hours: Math.max(1, Number(e.target.value) || 1) }))
              }
            />
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="tg-pass">{t('settings.telegram.passphrase')}</Label>
          <Input
            id="tg-pass"
            type="password"
            autoComplete="off"
            placeholder={data?.has_passphrase ? t('settings.telegram.storedPlaceholder') : t('settings.backup.min8Placeholder')}
            value={form.passphrase}
            onChange={(e) => setForm((f) => ({ ...f, passphrase: e.target.value }))}
          />
          <p className="text-xs text-muted-foreground">
            {t('settings.telegram.passphraseHint')}
          </p>
        </div>

        {data?.last_at && (
          <div className="rounded-lg border bg-muted/40 p-3 text-xs">
            <span className="text-muted-foreground">{t('settings.telegram.lastRun')} </span>
            <span className="font-medium">{new Date(data.last_at).toLocaleString()}</span>{' '}
            {data.last_ok ? (
              <Badge variant="success">{t('settings.telegram.statusSent')}</Badge>
            ) : (
              <Badge variant="destructive">{t('settings.telegram.statusFailed')}</Badge>
            )}
            {!data.last_ok && data.last_error && (
              <p className="mt-1 text-destructive">{data.last_error}</p>
            )}
          </div>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="outline"
            onClick={sendNow}
            disabled={runNow.isPending || !configured}
            title={configured ? undefined : t('settings.telegram.saveFirst')}
          >
            {runNow.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            {t('settings.telegram.sendTest')}
          </Button>
          <Button onClick={save} disabled={update.isPending}>
            {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
            {t('common.save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

// ---- SMTP (e-mail delivery) config ----
function SmtpCard() {
  const { data, isLoading } = useMailConfig()
  const update = useUpdateMailConfig()
  const test = useTestMail()
  const { toast } = useToast()
  const t = useT()

  const [form, setForm] = useState<MailInput>({
    enabled: false, provider: 'smtp', host: '', port: 587, username: '', password: '', from: '', security: 'starttls', resend_key: '',
  })
  const [testTo, setTestTo] = useState('')

  useEffect(() => {
    if (data) {
      setForm({
        enabled: data.enabled, provider: data.provider || 'smtp', host: data.host, port: data.port || 587,
        username: data.username, password: '', from: data.from, security: data.security, resend_key: '',
      })
    }
  }, [data])

  const isResend = form.provider === 'resend'

  function save() {
    update.mutate(form, {
      onSuccess: () => {
        toast({ title: t('settings.smtp.saved') })
        setForm((f) => ({ ...f, password: '' }))
      },
      onError: (err) =>
        toast({ variant: 'destructive', title: t('settings.saveFailed'), description: err instanceof RequestError ? err.message : t('common.tryAgain') }),
    })
  }

  function sendTest() {
    if (!testTo.trim()) return
    test.mutate(testTo.trim(), {
      onSuccess: () => toast({ title: t('settings.smtp.testSent') }),
      onError: (err) =>
        toast({ variant: 'destructive', title: t('settings.smtp.testFailed'), description: err instanceof RequestError ? err.message : t('common.tryAgain') }),
    })
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-40" />
          <Skeleton className="mt-2 h-4 w-64" />
        </CardHeader>
        <CardContent className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Mail className="h-5 w-5 text-primary" />
          {t('settings.smtp.title')}
        </CardTitle>
        <CardDescription>{t('settings.smtp.desc')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <p className="text-sm font-medium">{t('settings.smtp.enabled')}</p>
            <p className="text-xs text-muted-foreground">{t('settings.smtp.enabledHint')}</p>
          </div>
          <Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} />
        </div>

        <div className="space-y-2">
          <Label htmlFor="mail-provider">{t('settings.smtp.provider')}</Label>
          <select id="mail-provider" value={form.provider}
            onChange={(e) => setForm((f) => ({ ...f, provider: e.target.value as MailInput['provider'] }))}
            className="flex h-9 w-full cursor-pointer rounded-md border border-input bg-background px-3 py-1 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <option value="resend">Resend (HTTPS API)</option>
            <option value="smtp">SMTP</option>
          </select>
          <p className="text-xs text-muted-foreground">
            {isResend ? t('settings.smtp.providerResendHint') : t('settings.smtp.providerSmtpHint')}
          </p>
        </div>

        {isResend ? (
          <div className="space-y-2">
            <Label htmlFor="resend-key">{t('settings.smtp.resendKey')}</Label>
            <Input id="resend-key" type="password" autoComplete="off"
              placeholder={data?.has_resend_key ? t('settings.telegram.storedPlaceholder') : 're_…'}
              value={form.resend_key} onChange={(e) => setForm((f) => ({ ...f, resend_key: e.target.value }))} />
            <p className="text-xs text-muted-foreground">{t('settings.smtp.resendKeyHint')}</p>
          </div>
        ) : (
          <>
            <div className="grid gap-4 sm:grid-cols-[2fr_1fr]">
              <div className="space-y-2">
                <Label htmlFor="smtp-host">{t('settings.smtp.host')}</Label>
                <Input id="smtp-host" placeholder="smtp.gmail.com" value={form.host}
                  onChange={(e) => setForm((f) => ({ ...f, host: e.target.value }))} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="smtp-port">{t('settings.smtp.port')}</Label>
                <Input id="smtp-port" type="number" min={1} value={form.port}
                  onChange={(e) => setForm((f) => ({ ...f, port: Number(e.target.value) || 0 }))} />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="smtp-security">{t('settings.smtp.security')}</Label>
              <select id="smtp-security" value={form.security}
                onChange={(e) => setForm((f) => ({ ...f, security: e.target.value as MailInput['security'] }))}
                className="flex h-9 w-full cursor-pointer rounded-md border border-input bg-background px-3 py-1 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <option value="starttls">STARTTLS (587)</option>
                <option value="tls">TLS (465)</option>
                <option value="none">{t('settings.smtp.securityNone')}</option>
              </select>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="smtp-user">{t('settings.smtp.username')}</Label>
                <Input id="smtp-user" autoComplete="off" placeholder="you@gmail.com" value={form.username}
                  onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="smtp-pass">{t('settings.smtp.password')}</Label>
                <Input id="smtp-pass" type="password" autoComplete="off"
                  placeholder={data?.has_password ? t('settings.telegram.storedPlaceholder') : '••••••••'}
                  value={form.password} onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))} />
              </div>
            </div>
          </>
        )}

        <div className="space-y-2">
          <Label htmlFor="smtp-from">{t('settings.smtp.from')}</Label>
          <Input id="smtp-from" placeholder="ADP Panel <onboarding@resend.dev>" value={form.from}
            onChange={(e) => setForm((f) => ({ ...f, from: e.target.value }))} />
          <p className="text-xs text-muted-foreground">
            {isResend ? t('settings.smtp.fromResendHint') : t('settings.smtp.fromHint')}
          </p>
        </div>

        <div className="flex flex-col gap-2 rounded-lg border bg-muted/40 p-3 sm:flex-row sm:items-end">
          <div className="flex-1 space-y-2">
            <Label htmlFor="smtp-test">{t('settings.smtp.testTo')}</Label>
            <Input id="smtp-test" type="email" placeholder="me@example.com" value={testTo}
              onChange={(e) => setTestTo(e.target.value)} />
          </div>
          <Button variant="outline" onClick={sendTest} disabled={test.isPending || !testTo.trim() || !data?.enabled}
            title={data?.enabled ? undefined : t('settings.smtp.saveFirst')}>
            {test.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            {t('settings.smtp.sendTest')}
          </Button>
        </div>

        <div className="flex justify-end">
          <Button onClick={save} disabled={update.isPending}>
            {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
            {t('common.save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

// ---- E-mail auto-backup ----
function EmailBackupCard() {
  const { data, isLoading } = useEmailBackup()
  const update = useUpdateEmailBackup()
  const runNow = useRunEmailBackup()
  const { toast } = useToast()
  const t = useT()

  const [form, setForm] = useState<EmailBackupInput>({ enabled: false, to: '', passphrase: '', interval_hours: 24 })

  useEffect(() => {
    if (data) {
      setForm({ enabled: data.enabled, to: data.to, passphrase: '', interval_hours: data.interval_hours })
    }
  }, [data])

  const configured = !!data?.has_passphrase && !!data?.to && !!data?.smtp_ready

  function save() {
    update.mutate(form, {
      onSuccess: () => {
        toast({ title: t('settings.emailBackup.saved') })
        setForm((f) => ({ ...f, passphrase: '' }))
      },
      onError: (err) =>
        toast({ variant: 'destructive', title: t('settings.saveFailed'), description: err instanceof RequestError ? err.message : t('common.tryAgain') }),
    })
  }

  function sendNow() {
    runNow.mutate(undefined, {
      onSuccess: () => toast({ title: t('settings.emailBackup.sent') }),
      onError: (err) =>
        toast({ variant: 'destructive', title: t('settings.emailBackup.sendFailed'), description: err instanceof RequestError ? err.message : t('common.tryAgain') }),
    })
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className="h-6 w-48" />
          <Skeleton className="mt-2 h-4 w-64" />
        </CardHeader>
        <CardContent className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <DatabaseBackup className="h-5 w-5 text-primary" />
          {t('settings.emailBackup.title')}
        </CardTitle>
        <CardDescription>{t('settings.emailBackup.desc')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!data?.smtp_ready && (
          <div className="flex items-start gap-2 rounded-lg border border-warning/40 bg-warning/5 p-3 text-xs">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-warning" />
            <span>{t('settings.emailBackup.needSmtp')}</span>
          </div>
        )}

        <div className="flex items-center justify-between rounded-lg border p-3">
          <div>
            <p className="text-sm font-medium">{t('settings.emailBackup.scheduled')}</p>
            <p className="text-xs text-muted-foreground">{t('settings.emailBackup.scheduledHint')}</p>
          </div>
          <Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} />
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="eb-to">{t('settings.emailBackup.to')}</Label>
            <Input id="eb-to" type="email" placeholder="me@example.com" value={form.to}
              onChange={(e) => setForm((f) => ({ ...f, to: e.target.value }))} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="eb-interval">{t('settings.telegram.interval')}</Label>
            <Input id="eb-interval" type="number" min={1} value={form.interval_hours}
              onChange={(e) => setForm((f) => ({ ...f, interval_hours: Math.max(1, Number(e.target.value) || 1) }))} />
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="eb-pass">{t('settings.telegram.passphrase')}</Label>
          <Input id="eb-pass" type="password" autoComplete="off"
            placeholder={data?.has_passphrase ? t('settings.telegram.storedPlaceholder') : t('settings.backup.min8Placeholder')}
            value={form.passphrase} onChange={(e) => setForm((f) => ({ ...f, passphrase: e.target.value }))} />
          <p className="text-xs text-muted-foreground">{t('settings.telegram.passphraseHint')}</p>
        </div>

        {data?.last_at && (
          <div className="rounded-lg border bg-muted/40 p-3 text-xs">
            <span className="text-muted-foreground">{t('settings.telegram.lastRun')} </span>
            <span className="font-medium">{new Date(data.last_at).toLocaleString()}</span>{' '}
            {data.last_ok ? (
              <Badge variant="success">{t('settings.telegram.statusSent')}</Badge>
            ) : (
              <Badge variant="destructive">{t('settings.telegram.statusFailed')}</Badge>
            )}
            {!data.last_ok && data.last_error && <p className="mt-1 text-destructive">{data.last_error}</p>}
          </div>
        )}

        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="outline" onClick={sendNow} disabled={runNow.isPending || !configured}
            title={configured ? undefined : t('settings.emailBackup.saveFirst')}>
            {runNow.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            {t('settings.emailBackup.sendNow')}
          </Button>
          <Button onClick={save} disabled={update.isPending}>
            {update.isPending && <Loader2 className="h-4 w-4 animate-spin" />}
            {t('common.save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function TwoFactorDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const { toast } = useToast()
  const { refresh } = useAuth()
  const t = useT()

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
        if (!cancelled) setError(err instanceof RequestError ? err.message : t('settings.twofa.startFailed'))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, t])

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
      toast({ title: t('settings.twofa.enabled') })
      await refresh()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof RequestError ? err.message : t('settings.twofa.invalidCode'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('settings.twofa.dialogTitle')}</DialogTitle>
          <DialogDescription>
            {t('settings.twofa.dialogDesc')}
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
                alt={t('settings.twofa.qrAlt')}
                className="h-44 w-44 rounded-lg border bg-white p-2"
              />
            </div>

            {setup.secret && (
              <div className="space-y-1.5">
                <Label>{t('settings.twofa.secretKey')}</Label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 truncate rounded-md border bg-muted px-3 py-2 font-mono text-xs">
                    {setup.secret}
                  </code>
                  <Button type="button" variant="outline" size="icon" onClick={copySecret}>
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    <span className="sr-only">{t('settings.twofa.copySecret')}</span>
                  </Button>
                </div>
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="totp-code">{t('settings.twofa.authCode')}</Label>
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
              {t('common.enable')}
            </Button>
          </form>
        ) : (
          <p className="text-sm text-destructive">{error ?? t('settings.twofa.genericError')}</p>
        )}
      </DialogContent>
    </Dialog>
  )
}
