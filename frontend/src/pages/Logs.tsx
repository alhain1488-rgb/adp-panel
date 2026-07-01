import { useState } from 'react'
import { ScrollText, ChevronLeft, ChevronRight } from 'lucide-react'
import { PageHeader } from '@/components/common/page-header'
import { EmptyState } from '@/components/common/misc'
import { Button } from '@/components/ui/button'
import { Badge, type BadgeProps } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useLogs } from '@/api/hooks'
import type { AuditLog } from '@/api/types'
import { formatRelativeTime } from '@/lib/utils'

const PAGE_SIZE = 50

function actionVariant(action: string): BadgeProps['variant'] {
  const category = action.split(/[._-]/)[0]
  switch (category) {
    case 'login':
    case 'auth':
      return 'default'
    case 'server':
      return 'secondary'
    case 'inbound':
      return 'outline'
    case 'client':
      return 'success'
    case 'sync':
      return 'warning'
    case 'settings':
      return 'secondary'
    default:
      return 'outline'
  }
}

function detailString(detail: AuditLog['detail']): string | null {
  if (!detail || Object.keys(detail).length === 0) return null
  try {
    return JSON.stringify(detail)
  } catch {
    return null
  }
}

export default function LogsPage() {
  const [page, setPage] = useState(1)
  const { data, isLoading, isFetching } = useLogs(page, PAGE_SIZE)

  const total = data?.total ?? 0
  const pageSize = data?.page_size ?? PAGE_SIZE
  const currentPage = data?.page ?? page
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const items = data?.items ?? []

  return (
    <div>
      <PageHeader title="Logs" description="Audit log" />

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="space-y-3 p-4">
              {Array.from({ length: 8 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : items.length === 0 ? (
            <div className="p-6">
              <EmptyState
                icon={ScrollText}
                title="No activity yet"
                description="Admin actions will show up here as they happen."
              />
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[140px]">Time</TableHead>
                  <TableHead className="w-[160px]">Action</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Admin</TableHead>
                  <TableHead className="w-[140px]">IP</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((log) => {
                  const detail = detailString(log.detail)
                  return (
                    <TableRow key={log.id}>
                      <TableCell
                        className="whitespace-nowrap text-muted-foreground"
                        title={new Date(log.created_at).toLocaleString()}
                      >
                        {formatRelativeTime(log.created_at)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={actionVariant(log.action)} className="font-mono">
                          {log.action}
                        </Badge>
                        {detail && (
                          <p className="mt-1 max-w-[260px] truncate font-mono text-xs text-muted-foreground" title={detail}>
                            {detail}
                          </p>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {log.target_type ? (
                          <>
                            {log.target_type}
                            {log.target_id != null && <span className="text-foreground"> #{log.target_id}</span>}
                          </>
                        ) : (
                          '—'
                        )}
                      </TableCell>
                      <TableCell>{log.admin_username ?? '—'}</TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">{log.ip ?? '—'}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {items.length > 0 && (
        <div className="mt-4 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Page {currentPage} of {totalPages} · {total} entries
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={currentPage <= 1 || isFetching}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              <ChevronLeft className="h-4 w-4" />
              Prev
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={currentPage >= totalPages || isFetching}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
