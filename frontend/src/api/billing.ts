import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import { qk } from './hooks'

// Billing settings (operator-tunable). Amounts are in kopecks (100 = 1 RUB).
export interface BillingSettings {
  enabled: boolean
  tariff_week_kopecks: number
  tariff_month_kopecks: number
  tariff_year_kopecks: number
  star_rate_kopecks: number
  support_contact: string
}

// One wallet-ledger entry.
export interface BillingTransaction {
  id: number
  kind: string // topup | purchase | adjust | refund | grant
  method: string // stars | card | sbp | crypto | manual | wallet
  amount_kopecks: number // signed: +credit, -debit
  stars?: number
  tariff?: string
  detail?: string
  created_at: string
}

// A client's wallet + subscription snapshot.
export interface ClientBilling {
  client_id: number
  balance_kopecks: number
  active_until: string // RFC3339 or ""
  active: boolean
  managed: boolean
  transactions: BillingTransaction[]
}

// formatRubles renders kopecks as a ruble string, e.g. 20000 -> "200 ₽",
// 7050 -> "70,50 ₽", -20000 -> "−200 ₽".
export function formatRubles(kopecks: number): string {
  const neg = kopecks < 0
  const abs = Math.abs(kopecks)
  const rub = Math.floor(abs / 100)
  const kop = abs % 100
  const body = kop === 0 ? `${rub} ₽` : `${rub},${String(kop).padStart(2, '0')} ₽`
  return neg ? `−${body}` : body
}

const BILLING_SETTINGS_KEY = ['billing', 'settings']

export function useBillingSettings() {
  return useQuery({
    queryKey: BILLING_SETTINGS_KEY,
    queryFn: () => api.get<BillingSettings>('/api/billing/settings'),
  })
}

export function useUpdateBillingSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: BillingSettings) => api.put<BillingSettings>('/api/billing/settings', input),
    onSuccess: (data) => qc.setQueryData(BILLING_SETTINGS_KEY, data),
  })
}

export function clientBillingKey(id: number) {
  return [...qk.client(id), 'billing'] as const
}

export function useClientBilling(id: number, enabled = true) {
  return useQuery({
    queryKey: clientBillingKey(id),
    queryFn: () => api.get<ClientBilling>(`/api/clients/${id}/billing`),
    enabled: enabled && id > 0,
  })
}

// useClientTopup applies an operator wallet adjustment (kopecks; +credit/-debit).
export function useClientTopup(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { kopecks: number; detail?: string }) =>
      api.post<ClientBilling>(`/api/clients/${id}/billing/topup`, body),
    onSuccess: (data) => {
      qc.setQueryData(clientBillingKey(id), data)
      qc.invalidateQueries({ queryKey: qk.client(id) })
    },
  })
}

// useClientGrant extends a client's subscription for free (by tariff or days).
export function useClientGrant(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { tariff?: string; days?: number }) =>
      api.post<ClientBilling>(`/api/clients/${id}/billing/grant`, body),
    onSuccess: (data) => {
      qc.setQueryData(clientBillingKey(id), data)
      qc.invalidateQueries({ queryKey: qk.client(id) })
      qc.invalidateQueries({ queryKey: qk.clients })
    },
  })
}
