import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'

// One Tribute product mapped to the subscription it grants. Tribute fixes the
// price, so the panel only needs to know how many days the product is worth —
// which is also how Tribute can sell periods the panel's tariffs don't have.
export interface TributeProduct {
  product_id: number
  days: number
  title?: string
  link?: string
}

export interface TributeSettings {
  enabled: boolean
  has_api_key: boolean // the key itself is never sent to the browser
  products: TributeProduct[]
}

// TributeInput adds the write-only API key. Blank keeps the stored one.
export interface TributeInput extends TributeSettings {
  api_key?: string
}

const TRIBUTE_KEY = ['billing', 'tribute']

export function useTributeSettings() {
  return useQuery({
    queryKey: TRIBUTE_KEY,
    queryFn: () => api.get<TributeSettings>('/api/billing/tribute'),
  })
}

export function useUpdateTributeSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: TributeInput) => api.put<TributeSettings>('/api/billing/tribute', input),
    onSuccess: (data) => qc.setQueryData(TRIBUTE_KEY, data),
  })
}
