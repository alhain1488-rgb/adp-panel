import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
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
import { PROTOCOL_LABELS } from '@/api/types'
import type { Inbound, InboundInput, Protocol } from '@/api/types'
import { cn } from '@/lib/utils'

const PROTOCOLS = Object.keys(PROTOCOL_LABELS) as Protocol[]

const selectClass =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50'

function stringifyJson(value?: Record<string, unknown>): string {
  if (!value || Object.keys(value).length === 0) return ''
  return JSON.stringify(value, null, 2)
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
  const createInbound = useCreateInbound(serverId)
  const updateInbound = useUpdateInbound(serverId)

  const [tag, setTag] = useState('')
  const [protocol, setProtocol] = useState<Protocol>('vless')
  const [listen, setListen] = useState('0.0.0.0')
  const [port, setPort] = useState('')
  const [remark, setRemark] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [settingsText, setSettingsText] = useState('')
  const [streamText, setStreamText] = useState('')
  const [settingsError, setSettingsError] = useState<string | null>(null)
  const [streamError, setStreamError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    setTag(inbound?.tag ?? '')
    setProtocol(inbound?.protocol ?? 'vless')
    setListen(inbound?.listen ?? '0.0.0.0')
    setPort(inbound?.port != null ? String(inbound.port) : '')
    setRemark(inbound?.remark ?? '')
    setEnabled(inbound?.enabled ?? true)
    setSettingsText(stringifyJson(inbound?.settings))
    setStreamText(stringifyJson(inbound?.stream_settings))
    setSettingsError(null)
    setStreamError(null)
  }, [open, inbound])

  const pending = createInbound.isPending || updateInbound.isPending

  function parseJson(text: string): Record<string, unknown> | undefined {
    const trimmed = text.trim()
    if (!trimmed) return undefined
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const parsed = JSON.parse(trimmed) as any
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      throw new Error('Expected a JSON object')
    }
    return parsed as Record<string, unknown>
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSettingsError(null)
    setStreamError(null)

    let settings: Record<string, unknown> | undefined
    let stream_settings: Record<string, unknown> | undefined
    try {
      settings = parseJson(settingsText)
    } catch (err) {
      setSettingsError(err instanceof Error ? err.message : 'Invalid JSON')
      return
    }
    try {
      stream_settings = parseJson(streamText)
    } catch (err) {
      setStreamError(err instanceof Error ? err.message : 'Invalid JSON')
      return
    }

    const input: InboundInput = {
      tag: tag.trim(),
      protocol,
      listen: listen.trim() || '0.0.0.0',
      port: Number(port),
      remark: remark.trim() || undefined,
      enabled,
      settings,
      stream_settings,
    }

    try {
      if (isEdit && inbound) {
        await updateInbound.mutateAsync({ id: inbound.id, input })
        toast({ title: 'Inbound updated' })
      } else {
        await createInbound.mutateAsync(input)
        toast({ title: 'Inbound created' })
      }
      onOpenChange(false)
    } catch (err) {
      toast({
        variant: 'destructive',
        title: isEdit ? 'Failed to update inbound' : 'Failed to create inbound',
        description: err instanceof Error ? err.message : undefined,
      })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{isEdit ? 'Edit inbound' : 'Add inbound'}</DialogTitle>
            <DialogDescription>
              {isEdit ? 'Update this inbound configuration.' : 'Create a new inbound on this server.'}
            </DialogDescription>
          </DialogHeader>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="ib-tag">Tag</Label>
              <Input
                id="ib-tag"
                value={tag}
                onChange={(e) => setTag(e.target.value)}
                placeholder="vless-reality"
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ib-protocol">Protocol</Label>
              <select
                id="ib-protocol"
                className={selectClass}
                value={protocol}
                onChange={(e) => setProtocol(e.target.value as Protocol)}
              >
                {PROTOCOLS.map((p) => (
                  <option key={p} value={p}>
                    {PROTOCOL_LABELS[p]}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="ib-listen">Listen</Label>
              <Input
                id="ib-listen"
                value={listen}
                onChange={(e) => setListen(e.target.value)}
                placeholder="0.0.0.0"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ib-port">Port</Label>
              <Input
                id="ib-port"
                type="number"
                min={1}
                max={65535}
                value={port}
                onChange={(e) => setPort(e.target.value)}
                placeholder="443"
                required
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="ib-remark">Remark</Label>
            <Input
              id="ib-remark"
              value={remark}
              onChange={(e) => setRemark(e.target.value)}
              placeholder="Optional label"
            />
          </div>

          <div className="flex items-center justify-between rounded-md border p-3">
            <div>
              <Label htmlFor="ib-enabled">Enabled</Label>
              <p className="text-xs text-muted-foreground">Serve this inbound to clients.</p>
            </div>
            <Switch id="ib-enabled" checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <Separator />

          <div className="space-y-3">
            <div>
              <p className="text-sm font-medium">Advanced (settings JSON)</p>
              <p className="text-xs text-muted-foreground">
                Reality keys and client identifiers are generated by the backend — leave those out.
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="ib-settings">settings</Label>
              <Textarea
                id="ib-settings"
                value={settingsText}
                onChange={(e) => setSettingsText(e.target.value)}
                placeholder='{ "decryption": "none" }'
                rows={5}
                className={cn('font-mono text-xs', settingsError && 'border-destructive')}
                spellCheck={false}
              />
              {settingsError && <p className="text-xs text-destructive">{settingsError}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="ib-stream">stream_settings</Label>
              <Textarea
                id="ib-stream"
                value={streamText}
                onChange={(e) => setStreamText(e.target.value)}
                placeholder='{ "network": "tcp", "security": "reality" }'
                rows={5}
                className={cn('font-mono text-xs', streamError && 'border-destructive')}
                spellCheck={false}
              />
              {streamError && <p className="text-xs text-destructive">{streamError}</p>}
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {pending && <Loader2 className="h-4 w-4 animate-spin" />}
              {isEdit ? 'Save changes' : 'Create inbound'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
