import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'

// The global forbidden-domain list, applied at the xray/sing-box engine level.
export interface Blocklist {
  domains: string // raw, one domain per line
  count: number // valid, deduplicated domains
}

const BLOCKLIST_KEY = ['blocklist']

export function useBlocklist() {
  return useQuery({ queryKey: BLOCKLIST_KEY, queryFn: () => api.get<Blocklist>('/api/blocklist') })
}

export function useUpdateBlocklist() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (domains: string) => api.put<Blocklist>('/api/blocklist', { domains }),
    onSuccess: (data) => qc.setQueryData(BLOCKLIST_KEY, data),
  })
}
