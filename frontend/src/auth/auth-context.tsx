import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api, getToken, setToken } from '@/api/client'
import type { Admin, LoginResponse } from '@/api/types'

interface AuthContextValue {
  admin: Admin | null
  loading: boolean
  authenticated: boolean
  login: (username: string, password: string) => Promise<LoginResponse>
  verifyTotp: (code: string, challengeId?: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [admin, setAdmin] = useState<Admin | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setAdmin(null)
      setLoading(false)
      return
    }
    try {
      const me = await api.get<Admin>('/api/auth/me')
      setAdmin(me)
    } catch {
      setToken(null)
      setAdmin(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const login = useCallback(async (username: string, password: string) => {
    const res = await api.post<LoginResponse>('/api/auth/login', { username, password })
    if (!res.need_2fa && res.tokens?.token) {
      setToken(res.tokens.token)
      await refresh()
    }
    return res
  }, [refresh])

  const verifyTotp = useCallback(async (code: string, challengeId?: string) => {
    const tokens = await api.post<{ token: string }>('/api/auth/2fa/verify', {
      code,
      challenge_id: challengeId,
    })
    setToken(tokens.token)
    await refresh()
  }, [refresh])

  const logout = useCallback(async () => {
    try {
      await api.post('/api/auth/logout')
    } catch {
      /* ignore */
    }
    setToken(null)
    setAdmin(null)
  }, [])

  return (
    <AuthContext.Provider
      value={{ admin, loading, authenticated: !!admin, login, verifyTotp, logout, refresh }}
    >
      {children}
    </AuthContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
