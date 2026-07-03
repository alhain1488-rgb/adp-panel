import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { cn } from '@/lib/utils'
import { useT } from '@/i18n/i18n'

// Renders a QR code from arbitrary text on the client. The backend also exposes
// GET /api/clients/{id}/qrcode, but rendering locally keeps the detail view snappy.
export function QrCode({ text, size = 200, className }: { text: string; size?: number; className?: string }) {
  const t = useT()
  const [src, setSrc] = useState<string>('')

  useEffect(() => {
    let alive = true
    QRCode.toDataURL(text, { margin: 1, width: size * 2 })
      .then((url) => alive && setSrc(url))
      .catch(() => alive && setSrc(''))
    return () => {
      alive = false
    }
  }, [text, size])

  return (
    <div
      className={cn('inline-flex items-center justify-center rounded-lg bg-white p-3', className)}
      style={{ width: size, height: size }}
    >
      {src ? (
        <img src={src} alt={t('common.qrCode')} width={size - 24} height={size - 24} />
      ) : (
        <span className="text-xs text-muted-foreground">…</span>
      )}
    </div>
  )
}
