import { Cpu, MemoryStick, HardDrive, Clock, Server as ServerIcon, Activity } from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { useT } from '@/i18n/i18n'
import { useSystemInfo, type SystemInfo } from '@/api/system'

function formatBytes(n: number): string {
  if (!n || n < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

function formatUptime(secs: number): string {
  if (!secs || secs < 0) return '—'
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  const parts: string[] = []
  if (d) parts.push(`${d}d`)
  if (h || d) parts.push(`${h}h`)
  parts.push(`${m}m`)
  return parts.join(' ')
}

// barTone picks a colour for a usage bar: calm under 70%, amber to 90%, red above.
function barTone(pct: number): string {
  if (pct >= 90) return 'bg-destructive'
  if (pct >= 70) return 'bg-warning'
  return 'bg-primary'
}

function UsageBar({ pct }: { pct: number }) {
  const clamped = Math.max(0, Math.min(100, pct))
  return (
    <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
      <div
        className={cn('h-full rounded-full transition-all', barTone(clamped))}
        style={{ width: `${clamped}%` }}
      />
    </div>
  )
}

function MetricCard({
  icon: Icon,
  title,
  primary,
  secondary,
  pct,
  showBar = true,
}: {
  icon: typeof Cpu
  title: string
  primary: string
  secondary?: string
  pct: number
  showBar?: boolean
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Icon className="h-4 w-4" />
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="flex items-baseline justify-between gap-2">
          <span className="text-2xl font-semibold tabular-nums">{primary}</span>
          {secondary && <span className="text-xs text-muted-foreground">{secondary}</span>}
        </div>
        {showBar && <UsageBar pct={pct} />}
      </CardContent>
    </Card>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate font-mono text-xs">{value}</span>
    </div>
  )
}

function SystemBody({ info }: { info: SystemInfo }) {
  const t = useT()
  const memPct = info.mem_total_bytes ? (info.mem_used_bytes / info.mem_total_bytes) * 100 : 0
  const diskPct = info.disk_total_bytes ? (info.disk_used_bytes / info.disk_total_bytes) * 100 : 0
  const swapPct = info.swap_total_bytes ? (info.swap_used_bytes / info.swap_total_bytes) * 100 : 0
  // Load average relative to core count: 1.0-per-core ≈ 100% busy.
  const loadPct = info.cpu_cores ? (info.load1 / info.cpu_cores) * 100 : 0

  return (
    <div className="space-y-6">
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          icon={Activity}
          title={t('system.load')}
          primary={info.load1.toFixed(2)}
          secondary={t('system.cores', { n: info.cpu_cores })}
          pct={loadPct}
        />
        <MetricCard
          icon={MemoryStick}
          title={t('system.memory')}
          primary={`${Math.round(memPct)}%`}
          secondary={`${formatBytes(info.mem_used_bytes)} / ${formatBytes(info.mem_total_bytes)}`}
          pct={memPct}
        />
        <MetricCard
          icon={HardDrive}
          title={t('system.disk')}
          primary={`${Math.round(diskPct)}%`}
          secondary={`${formatBytes(info.disk_used_bytes)} / ${formatBytes(info.disk_total_bytes)}`}
          pct={diskPct}
        />
        <MetricCard
          icon={Clock}
          title={t('system.uptime')}
          primary={formatUptime(info.uptime_seconds)}
          showBar={false}
          pct={0}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base">
              <Cpu className="h-4 w-4" />
              {t('system.loadAverage')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <InfoRow label={t('system.load.1')} value={info.load1.toFixed(2)} />
            <InfoRow label={t('system.load.5')} value={info.load5.toFixed(2)} />
            <InfoRow label={t('system.load.15')} value={info.load15.toFixed(2)} />
            {info.swap_total_bytes > 0 && (
              <div className="mt-3 space-y-2 border-t pt-3">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">{t('system.swap')}</span>
                  <span className="font-mono text-xs">
                    {formatBytes(info.swap_used_bytes)} / {formatBytes(info.swap_total_bytes)}
                  </span>
                </div>
                <UsageBar pct={swapPct} />
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-base">
              <ServerIcon className="h-4 w-4" />
              {t('system.host')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {info.domain && <InfoRow label={t('system.domain')} value={info.domain} />}
            <InfoRow label={t('system.cpuCores')} value={String(info.cpu_cores)} />
            <InfoRow label={t('system.ram')} value={formatBytes(info.mem_total_bytes)} />
            <InfoRow label={t('system.diskTotal')} value={formatBytes(info.disk_total_bytes)} />
            <InfoRow label={t('system.kernel')} value={info.kernel || '—'} />
            <InfoRow label={t('system.arch')} value={info.arch || '—'} />
            <InfoRow label={t('system.panelVersion')} value={info.panel_version} />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

export default function SystemPage() {
  const t = useT()
  const { data, isLoading } = useSystemInfo()

  return (
    <div>
      <PageHeader title={t('system.title')} description={t('system.subtitle')} />
      {isLoading || !data ? (
        <div className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-28" />
            ))}
          </div>
          <div className="grid gap-4 lg:grid-cols-2">
            <Skeleton className="h-48" />
            <Skeleton className="h-48" />
          </div>
        </div>
      ) : (
        <SystemBody info={data} />
      )}
    </div>
  )
}
