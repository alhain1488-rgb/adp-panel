// Backup export/import use binary download and multipart upload, which the JSON
// `api` client doesn't cover — so these call fetch directly, reusing the same
// bearer token and error shape.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, getToken, RequestError } from './client'
import type { ApiError } from './types'

export interface ImportReport {
  servers: number
  inbounds: number
  clients: number
  source_version: string
  created_at: string
}

function authHeader(): Record<string, string> {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function toError(res: Response): Promise<RequestError> {
  let message = res.statusText
  let code: string | undefined
  if ((res.headers.get('content-type') ?? '').includes('application/json')) {
    const err = (await res.json().catch(() => null)) as ApiError | null
    if (err?.error) message = err.error
    code = err?.code
  }
  return new RequestError(res.status, message, code)
}

// downloadBackup requests an encrypted archive and saves it via a temporary link.
export async function downloadBackup(passphrase: string): Promise<void> {
  const res = await fetch('/api/backup/export', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeader() },
    body: JSON.stringify({ passphrase }),
  })
  if (!res.ok) throw await toError(res)

  const blob = await res.blob()
  const cd = res.headers.get('Content-Disposition') ?? ''
  const match = /filename="?([^"]+)"?/.exec(cd)
  const filename = match?.[1] ?? 'adp-panel-backup.adpbak'

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

// uploadBackup restores an archive; on success the panel restarts to apply it.
export async function uploadBackup(file: File, passphrase: string): Promise<ImportReport> {
  const form = new FormData()
  form.append('file', file)
  form.append('passphrase', passphrase)

  // Note: no Content-Type header — the browser sets the multipart boundary.
  const res = await fetch('/api/backup/import', {
    method: 'POST',
    headers: authHeader(),
    body: form,
  })
  if (!res.ok) throw await toError(res)

  const data = (await res.json()) as { report: ImportReport }
  return data.report
}

// ---- Telegram auto-backup ----

export interface TelegramStatus {
  enabled: boolean
  has_token: boolean
  chat_id: string
  has_passphrase: boolean
  interval_hours: number
  last_at: string
  last_error: string
  last_ok: boolean
}

export interface TelegramInput {
  enabled: boolean
  token: string // blank = keep the stored one
  chat_id: string
  passphrase: string // blank = keep the stored one
  interval_hours: number
}

const TG_KEY = ['backup', 'telegram']

export function useTelegramBackup() {
  return useQuery({
    queryKey: TG_KEY,
    queryFn: () => api.get<TelegramStatus>('/api/backup/telegram'),
  })
}

export function useUpdateTelegramBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: TelegramInput) => api.put<TelegramStatus>('/api/backup/telegram', input),
    onSuccess: (data) => qc.setQueryData(TG_KEY, data),
  })
}

export function useRunTelegramBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<{ ok: boolean }>('/api/backup/telegram/run'),
    // Refresh the status so the "last run" line updates after a send.
    onSuccess: () => qc.invalidateQueries({ queryKey: TG_KEY }),
  })
}

// ---- SMTP (e-mail) settings ----

export type MailProvider = 'smtp' | 'resend'

export interface MailStatus {
  enabled: boolean
  provider: MailProvider
  host: string
  port: number
  username: string
  has_password: boolean
  from: string
  security: 'starttls' | 'tls' | 'none'
  has_resend_key: boolean
}

export interface MailInput {
  enabled: boolean
  provider: MailProvider
  host: string
  port: number
  username: string
  password: string // blank = keep the stored one
  from: string
  security: 'starttls' | 'tls' | 'none'
  resend_key: string // blank = keep the stored one
}

const MAIL_KEY = ['mail']

export function useMailConfig() {
  return useQuery({ queryKey: MAIL_KEY, queryFn: () => api.get<MailStatus>('/api/mail') })
}

export function useUpdateMailConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: MailInput) => api.put<MailStatus>('/api/mail', input),
    onSuccess: (data) => qc.setQueryData(MAIL_KEY, data),
  })
}

export function useTestMail() {
  return useMutation({
    mutationFn: (to: string) => api.post<{ ok: boolean }>('/api/mail/test', { to }),
  })
}

// ---- E-mail auto-backup ----

export interface EmailBackupStatus {
  enabled: boolean
  to: string
  has_passphrase: boolean
  interval_hours: number
  smtp_ready: boolean
  last_at: string
  last_error: string
  last_ok: boolean
}

export interface EmailBackupInput {
  enabled: boolean
  to: string
  passphrase: string // blank = keep the stored one
  interval_hours: number
}

const EMAIL_KEY = ['backup', 'email']

export function useEmailBackup() {
  return useQuery({ queryKey: EMAIL_KEY, queryFn: () => api.get<EmailBackupStatus>('/api/backup/email') })
}

export function useUpdateEmailBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: EmailBackupInput) => api.put<EmailBackupStatus>('/api/backup/email', input),
    onSuccess: (data) => qc.setQueryData(EMAIL_KEY, data),
  })
}

export function useRunEmailBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<{ ok: boolean }>('/api/backup/email/run'),
    onSuccess: () => qc.invalidateQueries({ queryKey: EMAIL_KEY }),
  })
}
