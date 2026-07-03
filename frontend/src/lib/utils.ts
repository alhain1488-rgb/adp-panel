import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatRelativeTime(input?: string | null, lang: 'ru' | 'en' = 'en'): string {
  const ru = lang === 'ru'
  if (!input) return ru ? 'никогда' : 'never'
  const then = new Date(input).getTime()
  if (Number.isNaN(then)) return ru ? 'никогда' : 'never'
  const diff = Date.now() - then
  const sec = Math.round(diff / 1000)
  if (sec < 0) return ru ? 'только что' : 'just now'
  if (sec < 60) return ru ? `${sec} с назад` : `${sec}s ago`
  const min = Math.round(sec / 60)
  if (min < 60) return ru ? `${min} мин назад` : `${min}m ago`
  const hr = Math.round(min / 60)
  if (hr < 24) return ru ? `${hr} ч назад` : `${hr}h ago`
  const day = Math.round(hr / 24)
  return ru ? `${day} дн назад` : `${day}d ago`
}

export function formatBytes(mb?: number): string {
  if (mb == null) return '—'
  if (mb < 1024) return `${mb.toFixed(0)} MB`
  return `${(mb / 1024).toFixed(1)} GB`
}
