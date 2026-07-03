import { useEffect, useMemo, useState } from 'react'
import { Dices, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
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
import { useCreateInbound, useUpdateInbound } from '@/api/hooks'
import { useT } from '@/i18n/i18n'
import { PROTOCOL_LABELS } from '@/api/types'
import type { Inbound, InboundInput, Protocol } from '@/api/types'
import {
  buildConfig,
  emptyForm,
  FINGERPRINTS,
  NETWORKS,
  parseInbound,
  protocolDefaults,
  randomRealityTarget,
  securitiesFor,
  SNIFF_OVERRIDES,
  SS_METHODS,
  VMESS_CIPHERS,
  type InboundForm,
  type Network,
  type Security,
} from './inbound-defaults'

const selectClass =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50'

function Field({ label, htmlFor, hint, children }: { label: string; htmlFor?: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <p className="text-sm font-semibold">{children}</p>
}

export function InboundFormDialog({
  serverId,
  open,
  onOpenChange,
  inbound,
}: {
  serverId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  inbound?: Inbound
}) {
  const isEdit = !!inbound
  const { toast } = useToast()
  const t = useT()
  const createInbound = useCreateInbound(serverId)
  const updateInbound = useUpdateInbound(serverId)

  const [form, setForm] = useState<InboundForm>(emptyForm())

  useEffect(() => {
    if (!open) return
    if (inbound) {
      const base: InboundForm = {
        ...emptyForm(),
        tag: inbound.tag,
        protocol: inbound.protocol,
        listen: inbound.listen ?? '0.0.0.0',
        port: inbound.port != null ? String(inbound.port) : '',
        remark: inbound.remark ?? '',
        enabled: inbound.enabled ?? true,
      }
      setForm(parseInbound(base, inbound.settings, inbound.stream_settings, inbound.sniffing))
    } else {
      setForm({ ...emptyForm(), ...protocolDefaults('vless') })
    }
  }, [open, inbound])

  const patch = (p: Partial<InboundForm>) => setForm((f) => ({ ...f, ...p }))

  function changeProtocol(protocol: Protocol) {
    const defaults = protocolDefaults(protocol)
    const secs = securitiesFor(protocol)
    setForm((f) => ({
      ...f,
      protocol,
      ...defaults,
      security: secs.length ? (defaults.security ?? secs[0]) : f.security,
    }))
  }

  const preview = useMemo(() => JSON.stringify(buildConfig(form), null, 2), [form])
  const pending = createInbound.isPending || updateInbound.isPending
  const isXray = form.protocol === 'vless' || form.protocol === 'vmess' || form.protocol === 'trojan'
  const isAWG = form.protocol === 'amneziawg'
  const securities = securitiesFor(form.protocol)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const { settings, stream_settings, sniffing } = buildConfig(form)
    const input: InboundInput = {
      tag: form.tag.trim(),
      protocol: form.protocol,
      listen: form.listen.trim() || '0.0.0.0',
      port: Number(form.port),
      remark: form.remark.trim() || undefined,
      enabled: form.enabled,
      settings,
      stream_settings,
      sniffing,
    }
    try {
      if (isEdit && inbound) {
        await updateInbound.mutateAsync({ id: inbound.id, input })
        toast({ title: t('inboundForm.updated') })
      } else {
        await createInbound.mutateAsync(input)
        toast({ title: t('inboundForm.created') })
      }
      onOpenChange(false)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: isEdit ? t('inboundForm.updateFailed') : t('inboundForm.createFailed'),
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <form onSubmit={handleSubmit} className="space-y-5">
          <DialogHeader>
            <DialogTitle>{isEdit ? t('inboundForm.editTitle') : t('inboundForm.addTitle')}</DialogTitle>
            <DialogDescription>
              {t('inboundForm.desc')}
            </DialogDescription>
          </DialogHeader>

          {/* Basics */}
          <div className="grid grid-cols-2 gap-4">
            <Field label={t('inboundForm.tag')} htmlFor="ib-tag">
              <Input id="ib-tag" value={form.tag} onChange={(e) => patch({ tag: e.target.value })} placeholder="vless-reality" required />
            </Field>
            <Field label={t('inboundForm.protocol')} htmlFor="ib-protocol">
              <select id="ib-protocol" className={selectClass} value={form.protocol} onChange={(e) => changeProtocol(e.target.value as Protocol)}>
                {(Object.keys(PROTOCOL_LABELS) as Protocol[]).map((p) => (
                  <option key={p} value={p}>
                    {PROTOCOL_LABELS[p]}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t('inboundForm.listen')} htmlFor="ib-listen" hint={t('inboundForm.listenHint')}>
              <Input id="ib-listen" value={form.listen} onChange={(e) => patch({ listen: e.target.value })} placeholder="0.0.0.0" />
            </Field>
            <Field label={t('inboundForm.port')} htmlFor="ib-port">
              <Input id="ib-port" type="number" min={1} max={65535} value={form.port} onChange={(e) => patch({ port: e.target.value })} placeholder="443" required />
            </Field>
            <Field label={t('inboundForm.remark')} htmlFor="ib-remark">
              <Input id="ib-remark" value={form.remark} onChange={(e) => patch({ remark: e.target.value })} placeholder={t('inboundForm.remarkPlaceholder')} />
            </Field>
            <div className="flex items-end">
              <div className="flex w-full items-center justify-between rounded-md border px-3 py-2">
                <Label htmlFor="ib-enabled">{t('common.enabled')}</Label>
                <Switch id="ib-enabled" checked={form.enabled} onCheckedChange={(v) => patch({ enabled: v })} />
              </div>
            </div>
          </div>

          {/* Transport + security (xray protocols) */}
          {isXray && (
            <>
              <Separator />
              <SectionTitle>{t('inboundForm.transportSecurity')}</SectionTitle>
              <div className="grid grid-cols-2 gap-4">
                <Field label={t('inboundForm.network')} htmlFor="ib-network">
                  <select id="ib-network" className={selectClass} value={form.network} onChange={(e) => patch({ network: e.target.value as Network })}>
                    {NETWORKS.map((n) => (
                      <option key={n} value={n}>
                        {n}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label={t('inboundForm.security')} htmlFor="ib-security">
                  <select id="ib-security" className={selectClass} value={form.security} onChange={(e) => patch({ security: e.target.value as Security })}>
                    {securities.map((s) => (
                      <option key={s} value={s}>
                        {s}
                      </option>
                    ))}
                  </select>
                </Field>

                {form.protocol === 'vless' && (
                  <Field label={t('inboundForm.flow')} htmlFor="ib-flow">
                    <select id="ib-flow" className={selectClass} value={form.flow} onChange={(e) => patch({ flow: e.target.value })}>
                      <option value="">{t('inboundForm.none')}</option>
                      <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                    </select>
                  </Field>
                )}
                {form.protocol === 'vmess' && (
                  <Field label={t('inboundForm.cipher')} htmlFor="ib-cipher">
                    <select id="ib-cipher" className={selectClass} value={form.vmessCipher} onChange={(e) => patch({ vmessCipher: e.target.value })}>
                      {VMESS_CIPHERS.map((c) => (
                        <option key={c} value={c}>
                          {c}
                        </option>
                      ))}
                    </select>
                  </Field>
                )}
              </div>

              {/* Transport-specific */}
              {(form.network === 'ws' || form.network === 'httpupgrade' || form.network === 'xhttp') && (
                <div className="grid grid-cols-2 gap-4">
                  <Field label={t('inboundForm.path')} htmlFor="ib-path">
                    <Input id="ib-path" value={form.path} onChange={(e) => patch({ path: e.target.value })} placeholder="/" />
                  </Field>
                  <Field label={t('inboundForm.host')} htmlFor="ib-host" hint={t('inboundForm.hostHint')}>
                    <Input id="ib-host" value={form.host} onChange={(e) => patch({ host: e.target.value })} placeholder="example.com" />
                  </Field>
                </div>
              )}
              {form.network === 'grpc' && (
                <div className="grid grid-cols-2 gap-4">
                  <Field label={t('inboundForm.grpcServiceName')} htmlFor="ib-grpc">
                    <Input id="ib-grpc" value={form.grpcServiceName} onChange={(e) => patch({ grpcServiceName: e.target.value })} placeholder="grpc" />
                  </Field>
                  <div className="flex items-end">
                    <div className="flex w-full items-center justify-between rounded-md border px-3 py-2">
                      <Label htmlFor="ib-multimode">{t('inboundForm.multiMode')}</Label>
                      <Switch id="ib-multimode" checked={form.grpcMultiMode} onCheckedChange={(v) => patch({ grpcMultiMode: v })} />
                    </div>
                  </div>
                </div>
              )}
              {form.network === 'tcp' && (
                <Field label={t('inboundForm.tcpHeaderType')} htmlFor="ib-tcphdr">
                  <select id="ib-tcphdr" className={selectClass} value={form.tcpHeaderType} onChange={(e) => patch({ tcpHeaderType: e.target.value as 'none' | 'http' })}>
                    <option value="none">{t('inboundForm.none')}</option>
                    <option value="http">{t('inboundForm.httpCamouflage')}</option>
                  </select>
                </Field>
              )}

              {/* Security-specific */}
              {form.security === 'tls' && (
                <div className="grid grid-cols-2 gap-4">
                  <Field label={t('inboundForm.sniServerName')} htmlFor="ib-sni">
                    <Input id="ib-sni" value={form.sni} onChange={(e) => patch({ sni: e.target.value })} placeholder="example.com" />
                  </Field>
                  <Field label="ALPN" htmlFor="ib-alpn" hint={t('inboundForm.commaSeparated')}>
                    <Input id="ib-alpn" value={form.alpn} onChange={(e) => patch({ alpn: e.target.value })} placeholder="h2,http/1.1" />
                  </Field>
                  <Field label={t('inboundForm.fingerprintUtls')} htmlFor="ib-fp">
                    <select id="ib-fp" className={selectClass} value={form.fingerprint} onChange={(e) => patch({ fingerprint: e.target.value })}>
                      {FINGERPRINTS.map((fp) => (
                        <option key={fp} value={fp}>
                          {fp}
                        </option>
                      ))}
                    </select>
                  </Field>
                </div>
              )}
              {form.security === 'reality' && (
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-muted-foreground">{t('inboundForm.steeringTarget')}</p>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => patch(randomRealityTarget())}
                    >
                      <Dices className="h-4 w-4" />
                      {t('inboundForm.randomSni')}
                    </Button>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <Field label={t('inboundForm.destTarget')} htmlFor="ib-rdest">
                      <Input id="ib-rdest" value={form.realityDest} onChange={(e) => patch({ realityDest: e.target.value })} placeholder="www.yahoo.com:443" />
                    </Field>
                    <Field label={t('inboundForm.serverNamesSni')} htmlFor="ib-rsni" hint={t('inboundForm.commaSeparated')}>
                      <Input id="ib-rsni" value={form.realityServerNames} onChange={(e) => patch({ realityServerNames: e.target.value })} placeholder="www.yahoo.com" />
                    </Field>
                    <Field label={t('inboundForm.fingerprint')} htmlFor="ib-rfp">
                      <select id="ib-rfp" className={selectClass} value={form.fingerprint} onChange={(e) => patch({ fingerprint: e.target.value })}>
                        {FINGERPRINTS.map((fp) => (
                          <option key={fp} value={fp}>
                            {fp}
                          </option>
                        ))}
                      </select>
                    </Field>
                    <Field label="SpiderX" htmlFor="ib-spx">
                      <Input id="ib-spx" value={form.realitySpiderX} onChange={(e) => patch({ realitySpiderX: e.target.value })} placeholder="/" />
                    </Field>
                    <Field label={t('inboundForm.shortIds')} htmlFor="ib-rsid" hint={t('inboundForm.shortIdsHint')}>
                      <Input id="ib-rsid" value={form.realityShortIds} onChange={(e) => patch({ realityShortIds: e.target.value })} placeholder="auto" />
                    </Field>
                  </div>
                  <p className="rounded-md border border-primary/30 bg-primary/5 p-2 text-xs text-muted-foreground">
                    {t('inboundForm.realityKeypairNote')}
                  </p>
                </div>
              )}
            </>
          )}

          {/* Shadowsocks */}
          {form.protocol === 'shadowsocks' && (
            <>
              <Separator />
              <SectionTitle>Shadowsocks</SectionTitle>
              <div className="grid grid-cols-2 gap-4">
                <Field label={t('inboundForm.method')} htmlFor="ib-ssm">
                  <select id="ib-ssm" className={selectClass} value={form.ssMethod} onChange={(e) => patch({ ssMethod: e.target.value })}>
                    {SS_METHODS.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label={t('inboundForm.networkPlain')} htmlFor="ib-ssn">
                  <select id="ib-ssn" className={selectClass} value={form.ssNetwork} onChange={(e) => patch({ ssNetwork: e.target.value })}>
                    <option value="tcp,udp">tcp,udp</option>
                    <option value="tcp">tcp</option>
                    <option value="udp">udp</option>
                  </select>
                </Field>
              </div>
            </>
          )}

          {/* Hysteria2 (sing-box) */}
          {form.protocol === 'hysteria2' && (
            <>
              <Separator />
              <SectionTitle>Hysteria2 (sing-box)</SectionTitle>
              <div className="grid grid-cols-2 gap-4">
                <Field label={t('inboundForm.upMbps')} htmlFor="ib-hyup">
                  <Input id="ib-hyup" type="number" min={0} value={form.hyUp} onChange={(e) => patch({ hyUp: e.target.value })} />
                </Field>
                <Field label={t('inboundForm.downMbps')} htmlFor="ib-hydown">
                  <Input id="ib-hydown" type="number" min={0} value={form.hyDown} onChange={(e) => patch({ hyDown: e.target.value })} />
                </Field>
                <Field label={t('inboundForm.obfs')} htmlFor="ib-hyobfs">
                  <select id="ib-hyobfs" className={selectClass} value={form.hyObfsType} onChange={(e) => patch({ hyObfsType: e.target.value as 'none' | 'salamander' })}>
                    <option value="none">{t('inboundForm.none')}</option>
                    <option value="salamander">salamander</option>
                  </select>
                </Field>
                {form.hyObfsType === 'salamander' && (
                  <Field label={t('inboundForm.obfsPassword')} htmlFor="ib-hyobfspw" hint={t('inboundForm.blankGenerated')}>
                    <Input id="ib-hyobfspw" value={form.hyObfsPassword} onChange={(e) => patch({ hyObfsPassword: e.target.value })} />
                  </Field>
                )}
                <Field label="SNI" htmlFor="ib-hysni">
                  <Input id="ib-hysni" value={form.sni} onChange={(e) => patch({ sni: e.target.value })} placeholder={t('inboundForm.nodeHostname')} />
                </Field>
                <div className="flex items-end">
                  <div className="flex w-full items-center justify-between rounded-md border px-3 py-2">
                    <Label htmlFor="ib-hyinsecure">{t('inboundForm.tlsInsecure')}</Label>
                    <Switch id="ib-hyinsecure" checked={form.hyInsecure} onCheckedChange={(v) => patch({ hyInsecure: v })} />
                  </div>
                </div>
              </div>
              <p className="rounded-md border border-primary/30 bg-primary/5 p-2 text-xs text-muted-foreground">
                {t('inboundForm.hysteriaNote')}
              </p>
            </>
          )}

          {/* AmneziaWG */}
          {isAWG && (
            <>
              <Separator />
              <SectionTitle>AmneziaWG 2.0</SectionTitle>
              <p className="rounded-md border border-primary/30 bg-primary/5 p-3 text-xs text-muted-foreground">
                {t('inboundForm.awgNote')}
              </p>
            </>
          )}

          {/* Sniffing (not applicable to AmneziaWG) */}
          {!isAWG && (
            <>
              <Separator />
              <div className="flex items-center justify-between">
                <SectionTitle>{t('inboundForm.sniffing')}</SectionTitle>
                <Switch checked={form.sniffEnabled} onCheckedChange={(v) => patch({ sniffEnabled: v })} aria-label={t('inboundForm.sniffingEnabled')} />
              </div>
              {form.sniffEnabled && (
            <div className="flex flex-wrap gap-4">
              {SNIFF_OVERRIDES.map((o) => (
                <label key={o} className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={form.sniffOverrides.includes(o)}
                    onCheckedChange={(v) =>
                      patch({
                        sniffOverrides: v
                          ? [...form.sniffOverrides, o]
                          : form.sniffOverrides.filter((x) => x !== o),
                      })
                    }
                  />
                  {o}
                </label>
              ))}
            </div>
              )}
            </>
          )}

          {/* Live preview of the generated config (xray/hysteria only) */}
          {!isAWG && (
            <>
              <Separator />
              <div className="space-y-1.5">
                <SectionTitle>{t('inboundForm.generatedConfig')}</SectionTitle>
                <pre className="max-h-56 overflow-auto rounded-md border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
                  {preview}
                </pre>
              </div>
            </>
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending && <Loader2 className="h-4 w-4 animate-spin" />}
              {isEdit ? t('inboundForm.saveChanges') : t('inboundForm.createInbound')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
