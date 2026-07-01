import { Link } from 'react-router-dom'
import { Server, Users, Activity, RefreshCw, Boxes } from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { StatCard, UsageBar, EmptyState } from '@/components/common/misc'
import { StatusBadge, EngineBadge } from '@/components/common/badges'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useDashboard } from '@/api/hooks'
import { formatRelativeTime } from '@/lib/utils'

export default function DashboardPage() {
  const { data, isLoading } = useDashboard()

  return (
    <div>
      <PageHeader title="Dashboard" description="Overview of your servers and clients" />

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {isLoading || !data ? (
          Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-24" />)
        ) : (
          <>
            <StatCard
              label="Servers"
              value={data.server_count}
              icon={Server}
              hint={`${data.online_server_count} online`}
            />
            <StatCard
              label="Clients"
              value={data.client_count}
              icon={Users}
              hint={`${data.enabled_client_count} enabled`}
            />
            <StatCard label="Inbounds" value={data.inbound_count} icon={Boxes} />
            <StatCard
              label="Last sync"
              value={formatRelativeTime(data.last_sync_at)}
              icon={RefreshCw}
            />
          </>
        )}
      </div>

      <h2 className="mb-4 mt-8 text-lg font-semibold">Servers</h2>

      {isLoading || !data ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-64" />
          ))}
        </div>
      ) : data.servers.length === 0 ? (
        <EmptyState icon={Server} title="No servers yet" description="Add a server to start managing inbounds." />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {data.servers.map((s) => (
            <Link key={s.id} to={`/servers/${s.id}`} className="group">
              <Card className="h-full transition-colors group-hover:border-primary/40">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <CardTitle className="truncate text-base">{s.name}</CardTitle>
                      <p className="mt-0.5 truncate text-xs text-muted-foreground">
                        {s.geo_country ? `${s.geo_country} · ` : ''}
                        {s.host}
                      </p>
                    </div>
                    <StatusBadge status={s.status} />
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex flex-wrap gap-1.5">
                    {s.engines.length === 0 ? (
                      <span className="text-xs text-muted-foreground">No engines detected</span>
                    ) : (
                      s.engines.map((e) => (
                        <EngineBadge key={e.engine} engine={e.engine} running={e.running} />
                      ))
                    )}
                  </div>
                  <div className="space-y-2">
                    <UsageBar label="CPU" percent={s.stats.cpu_percent} />
                    <UsageBar
                      label="RAM"
                      percent={s.stats.mem_percent}
                      detail={`${(s.stats.mem_used_mb / 1024).toFixed(1)}/${(s.stats.mem_total_mb / 1024).toFixed(1)} GB`}
                    />
                    <UsageBar
                      label="Disk"
                      percent={s.stats.disk_percent}
                      detail={`${s.stats.disk_used_gb.toFixed(0)}/${s.stats.disk_total_gb.toFixed(0)} GB`}
                    />
                  </div>
                  <div className="flex items-center gap-1.5 pt-1 text-xs text-muted-foreground">
                    <Activity className="h-3.5 w-3.5" />
                    Synced {formatRelativeTime(s.last_sync_at)}
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
