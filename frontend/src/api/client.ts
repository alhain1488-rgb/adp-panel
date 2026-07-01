// Thin typed fetch wrapper. In Phase 1 the MSW worker intercepts these requests
// and answers from mock data; in Phase 7 the same calls hit the real backend
// through Caddy. The surface is identical because both honor docs/openapi.yaml.
import type { ApiError } from './types'

const TOKEN_KEY = 'xray_panel_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class RequestError extends Error {
  status: number
  code?: string
  constructor(status: number, message: string, code?: string) {
    super(message)
    this.name = 'RequestError'
    this.status = status
    this.code = code
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  signal?: AbortSignal
  headers?: Record<string, string>
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { ...opts.headers }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  let body: BodyInit | undefined
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(path, { method: opts.method ?? 'GET', headers, body, signal: opts.signal })

  if (res.status === 204) return undefined as T

  const contentType = res.headers.get('content-type') ?? ''
  if (!res.ok) {
    let message = res.statusText
    let code: string | undefined
    if (contentType.includes('application/json')) {
      const err = (await res.json().catch(() => null)) as ApiError | null
      if (err?.error) message = err.error
      code = err?.code
    }
    throw new RequestError(res.status, message, code)
  }

  if (contentType.includes('application/json')) return (await res.json()) as T
  return (await res.text()) as unknown as T
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body }),
  put: <T>(path: string, body?: unknown) => request<T>(path, { method: 'PUT', body }),
  del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
}
