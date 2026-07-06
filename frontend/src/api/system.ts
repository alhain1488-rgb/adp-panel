import { useQuery } from '@tanstack/react-query'
import { api } from './client'

// SystemInfo mirrors the backend systemDTO: host metrics for the machine the
// panel runs on, plus panel identity. On Linux the /proc-derived values reflect
// the host even though the backend runs in a container.
export interface SystemInfo {
  kernel: string
  arch: string
  cpu_cores: number
  load1: number
  load5: number
  load15: number
  mem_total_bytes: number
  mem_used_bytes: number
  mem_available_bytes: number
  swap_total_bytes: number
  swap_used_bytes: number
  disk_total_bytes: number
  disk_used_bytes: number
  disk_free_bytes: number
  uptime_seconds: number
  panel_version: string
  domain?: string
}

export function useSystemInfo() {
  return useQuery({
    queryKey: ['system'],
    queryFn: () => api.get<SystemInfo>('/api/system'),
    refetchInterval: 4000,
  })
}
