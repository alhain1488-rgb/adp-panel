import { useEffect, useState } from 'react'
import { Monitor, Moon, Sun, ShieldCheck, KeyRound, Loader2, Copy, Check } from 'lucide-react'
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
