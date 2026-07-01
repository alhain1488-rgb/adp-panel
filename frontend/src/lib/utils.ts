import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatRelativeTime(input?: string | null): string {
  if (!input) return 'never'
  const then = new Date(input).getTime()
  if (Number.isNaN(then)) return 'never'
  const diff = Date.now() - then
  const sec = Math.round(diff / 1000)
  if (sec < 0) return 'just now'
  if (sec < 60) return `${sec}s ago`
  const min = Math.round(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.round(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.round(hr / 24)
  return `${day}d ago`
}

export function formatBytes(mb?: number): string {
  if (mb == null) return '—'
  if (mb < 1024) return `${mb.toFixed(0)} MB`
  return `${(mb / 1024).toFixed(1)} GB`
}
