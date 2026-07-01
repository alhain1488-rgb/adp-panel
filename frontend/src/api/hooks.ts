import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type {
  Admin,
  AuditLogPage,
  Client,
  ClientInput,
  ClientLink,
  DashboardSummary,
  Inbound,
  InboundInput,
  Server,
  ServerInput,
  ServerStats,
  Settings,
} from './types'

export const qk = {
  me: ['me'] as const,
  dashboard: ['dashboard'] as const,
  servers: ['servers'] as const,
  server: (id: number) => ['servers', id] as const,
  serverStats: (id: number) => ['servers', id, 'stats'] as const,
  serverInbounds: (id: number) => ['servers', id, 'inbounds'] as const,
  inbound: (id: number) => ['inbounds', id] as const,
  clients: ['clients'] as const,
  client: (id: number) => ['clients', id] as const,
  clientLinks: (id: number) => ['clients', id, 'links'] as const,
  logs: (page: number) => ['logs', page] as const,
  settings: ['settings'] as const,
}

// ---- Auth ----
export function useMe(enabled = true) {
  return useQuery({ queryKey: qk.me, queryFn: () => api.get<Admin>('/api/auth/me'), enabled, retry: false })
}

// ---- Dashboard ----
export function useDashboard() {
  return useQuery({
    queryKey: qk.dashboard,
    queryFn: () => api.get<DashboardSummary>('/api/dashboard/summary'),
    refetchInterval: 15_000,
  })
}

// ---- Servers ----
export function useServers() {
  return useQuery({
    queryKey: qk.servers,
    queryFn: () => api.get<Server[]>('/api/servers'),
    // Poll while any node is still provisioning so installing → installed shows up.
    refetchInterval: (query) => {
      const data = query.state.data as Server[] | undefined
      const busy = data?.some(
        (s) => s.provision_status === 'installing' || s.provision_status === 'pending',
      )
      return busy ? 2500 : false
    },
  })
}
export function useServer(id: number) {
  return useQuery({ queryKey: qk.server(id), queryFn: () => api.get<Server>(`/api/servers/${id}`), enabled: id > 0 })
}
export function useServerStats(id: number) {
  return useQuery({
    queryKey: qk.serverStats(id),
    queryFn: () => api.get<ServerStats>(`/api/servers/${id}/stats`),
    enabled: id > 0,
  })
}
export function useCreateServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: ServerInput) => api.post<Server>('/api/servers', input),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.servers }),
  })
}
export function useUpdateServer(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: ServerInput) => api.put<Server>(`/api/servers/${id}`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.servers })
      qc.invalidateQueries({ queryKey: qk.server(id) })
    },
  })
}
export function useDeleteServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.del<void>(`/api/servers/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.servers }),
  })
}
export function useCheckServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post<Server>(`/api/servers/${id}/check`),
    onSuccess: (s) => {
      qc.invalidateQueries({ queryKey: qk.servers })
      qc.invalidateQueries({ queryKey: qk.server(s.id) })
      qc.invalidateQueries({ queryKey: qk.dashboard })
    },
  })
}
export function useInstallServer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.post<Server>(`/api/servers/${id}/install`),
    onSuccess: (s) => {
      qc.invalidateQueries({ queryKey: qk.servers })
      qc.invalidateQueries({ queryKey: qk.server(s.id) })
    },
  })
}

export function useRestartEngine(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (engine?: string) => api.post(`/api/servers/${id}/restart-xray`, engine ? { engine } : undefined),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.server(id) }),
  })
}

// ---- Inbounds ----
export function useServerInbounds(serverId: number) {
  return useQuery({
    queryKey: qk.serverInbounds(serverId),
    queryFn: () => api.get<Inbound[]>(`/api/servers/${serverId}/inbounds`),
    enabled: serverId > 0,
  })
}
export function useCreateInbound(serverId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: InboundInput) => api.post<Inbound>(`/api/servers/${serverId}/inbounds`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverInbounds(serverId) })
      qc.invalidateQueries({ queryKey: qk.server(serverId) })
    },
  })
}
export function useUpdateInbound(serverId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: InboundInput }) =>
      api.put<Inbound>(`/api/inbounds/${id}`, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.serverInbounds(serverId) }),
  })
}
export function useDeleteInbound(serverId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.del<void>(`/api/inbounds/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverInbounds(serverId) })
      qc.invalidateQueries({ queryKey: qk.server(serverId) })
    },
  })
}

// ---- Clients ----
export function useClients() {
  return useQuery({ queryKey: qk.clients, queryFn: () => api.get<Client[]>('/api/clients') })
}
export function useClient(id: number) {
  return useQuery({ queryKey: qk.client(id), queryFn: () => api.get<Client>(`/api/clients/${id}`), enabled: id > 0 })
}
export function useClientLinks(id: number) {
  return useQuery({
    queryKey: qk.clientLinks(id),
    queryFn: () => api.get<ClientLink[]>(`/api/clients/${id}/links`),
    enabled: id > 0,
  })
}
export function useCreateClient() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: ClientInput) => api.post<Client>('/api/clients', input),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.clients }),
  })
}
export function useUpdateClient(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: ClientInput) => api.put<Client>(`/api/clients/${id}`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.clients })
      qc.invalidateQueries({ queryKey: qk.client(id) })
    },
  })
}
export function useDeleteClient() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => api.del<void>(`/api/clients/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.clients }),
  })
}
export function useToggleClient(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (enabled: boolean) => api.post<Client>(`/api/clients/${id}/${enabled ? 'enable' : 'disable'}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.clients })
      qc.invalidateQueries({ queryKey: qk.client(id) })
    },
  })
}
export function useSetClientInbounds(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (inbound_ids: number[]) => api.put<Client>(`/api/clients/${id}/inbounds`, { inbound_ids }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.client(id) })
      qc.invalidateQueries({ queryKey: qk.clientLinks(id) })
    },
  })
}
export function useRotateToken(id: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<Client>(`/api/clients/${id}/rotate-token`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.client(id) })
      qc.invalidateQueries({ queryKey: qk.clientLinks(id) })
    },
  })
}

// ---- Logs ----
export function useLogs(page: number, pageSize = 50) {
  return useQuery({
    queryKey: qk.logs(page),
    queryFn: () => api.get<AuditLogPage>(`/api/logs?page=${page}&page_size=${pageSize}`),
  })
}

// ---- Settings ----
export function useSettings() {
  return useQuery({ queryKey: qk.settings, queryFn: () => api.get<Settings>('/api/settings') })
}
export function useUpdateSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: Settings) => api.put<Settings>('/api/settings', input),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.settings }),
  })
}
