import { useState } from 'react'
import { Loader2, LogOut, Download, QrCode as QrCodeIcon } from 'lucide-react'
import { DisgustingLogo } from '@/components/layout/logo'
import { LanguageToggle } from '@/components/layout/language-toggle'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { ProtocolBadge } from '@/components/common/badges'
import { CopyButton } from '@/components/common/copy-button'
import { QrCode } from '@/components/common/qr-code'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useT } from '@/i18n/i18n'
import { portalLogin, type PortalData, type PortalLink, type PortalAWG } from '@/api/portal'

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

function PortalLoginForm({ onSuccess }: { onSuccess: (d: PortalData) => void }) {
  const t = useT()
  const [name, setName] = useState('')
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !token.trim()) return
    setLoading(true)
    setError('')
    try {
      onSuccess(await portalLogin(name.trim(), token.trim()))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('portal.error'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card className="mx-auto max-w-md p-6">
      <h1 className="text-xl font-semibold">{t('portal.login.title')}</h1>
      <p className="mt-1 text-sm text-muted-foreground">{t('portal.login.desc')}</p>
      <form onSubmit={submit} className="mt-5 space-y-4">
        <div className="space-y-2">
          <Label htmlFor="portal-name">{t('portal.name')}</Label>
          <Input id="portal-name" autoFocus value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div className="space-y-2">
          <Label htmlFor="portal-token">{t('portal.token')}</Label>
          <Input
            id="portal-token"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder={t('portal.token.placeholder')}
            className="font-mono text-xs"
            required
          />
          <p className="text-xs text-muted-foreground">{t('portal.token.hint')}</p>
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button type="submit" className="w-full" disabled={loading || !name.trim() || !token.trim()}>
          {loading && <Loader2 className="h-4 w-4 animate-spin" />}
          {t('portal.login.button')}
        </Button>
      </form>
    </Card>
  )
}

function PortalLinkRow({ link }: { link: PortalLink }) {
  const t = useT()
  const [qrOpen, setQrOpen] = useState(false)
  return (
    <>
      <div className="flex items-center gap-3 px-4 py-3">
        <ProtocolBadge protocol={link.protocol} />
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{link.server_name ?? '—'}</div>
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
              {link.server_name ?? '—'}
            </DialogTitle>
            <DialogDescription className="truncate font-mono text-[11px]">{link.uri}</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col items-center gap-3 pb-2">
            <QrCode text={link.uri} size={220} />
            <CopyButton value={link.uri} size="sm" label={t('portal.copyLink')} />
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

function PortalAWGCard({ cfg }: { cfg: PortalAWG }) {
  const t = useT()
  const tag = cfg.tag || 'amneziawg'
  return (
    <Card className="p-4">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1 space-y-3">
          <div className="text-sm font-medium">
            {cfg.server_name} · <span className="font-mono">{tag}</span>
          </div>
          <div className="space-y-1.5">
            <div className="font-mono text-[11px] uppercase tracking-wide text-muted-foreground">vpn://</div>
            <div className="flex items-center gap-2">
              <Input readOnly value={cfg.vpn_link} onFocus={(e) => e.currentTarget.select()} className="font-mono text-xs" />
              <CopyButton value={cfg.vpn_link} className="shrink-0" />
            </div>
          </div>
          <Button variant="outline" size="sm" onClick={() => downloadText(`${tag}.conf`, cfg.conf)}>
            <Download className="h-4 w-4" />
            {t('portal.awg.download')}
          </Button>
        </div>
        <div className="shrink-0 self-center">
          <QrCode text={cfg.vpn_link} size={150} />
        </div>
      </div>
    </Card>
  )
}

function PortalView({ data, onExit }: { data: PortalData; onExit: () => void }) {
  const t = useT()
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <h1 className="truncate text-xl font-semibold">{t('portal.hello', { name: data.name })}</h1>
        <Button variant="outline" size="sm" onClick={onExit}>
          <LogOut className="h-4 w-4" />
          {t('portal.exit')}
        </Button>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t('portal.subscription')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-6 md:grid-cols-[1fr_auto]">
            <div className="space-y-3">
              <div className="flex gap-2">
                <Input
                  readOnly
                  value={data.subscription_url}
                  className="font-mono text-xs"
                  onFocus={(e) => e.currentTarget.select()}
                />
                <CopyButton value={data.subscription_url} />
              </div>
              <p className="text-xs text-muted-foreground">{t('portal.subscription.hint')}</p>
            </div>
            <div className="flex justify-center md:justify-end">
              {data.subscription_url && <QrCode text={data.subscription_url} size={190} />}
            </div>
          </div>
        </CardContent>
      </Card>

      {data.links.length > 0 && (
        <div>
          <h2 className="mb-3 text-sm font-medium">{t('portal.links')}</h2>
          <Card className="divide-y p-0">
            {data.links.map((l) => (
              <PortalLinkRow key={l.inbound_id} link={l} />
            ))}
          </Card>
        </div>
      )}

      {data.amneziawg && data.amneziawg.length > 0 && (
        <div>
          <h2 className="mb-1 text-sm font-medium">{t('portal.awg.title')}</h2>
          <p className="mb-3 text-xs text-muted-foreground">{t('portal.awg.hint')}</p>
          <div className="space-y-3">
            {data.amneziawg.map((c) => (
              <PortalAWGCard key={c.inbound_id} cfg={c} />
            ))}
          </div>
        </div>
      )}

      {data.links.length === 0 && !(data.amneziawg && data.amneziawg.length > 0) && (
        <p className="text-sm text-muted-foreground">{t('portal.empty')}</p>
      )}
    </div>
  )
}

export default function PortalPage() {
  const [data, setData] = useState<PortalData | null>(null)
  return (
    <div className="min-h-screen bg-background">
      <header className="flex h-16 items-center justify-between px-4 sm:px-6">
        <div className="flex items-center gap-3">
          <DisgustingLogo className="h-10 w-10" />
          <span className="text-base font-semibold tracking-tight">Absolutely Disgusting Panel</span>
        </div>
        <div className="flex items-center gap-1">
          <LanguageToggle />
          <ThemeToggle />
        </div>
      </header>
      <main className="mx-auto max-w-3xl px-4 py-8 sm:py-12">
        {data ? <PortalView data={data} onExit={() => setData(null)} /> : <PortalLoginForm onSuccess={setData} />}
      </main>
    </div>
  )
}
