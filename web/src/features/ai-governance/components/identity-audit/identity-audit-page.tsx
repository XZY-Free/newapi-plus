/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { Eye } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useMediaQuery } from '@/hooks'
import { formatTimestamp } from '@/lib/format'

import { listIdentityAuditEvents } from '../../api'
import { useGovernanceTableState } from '../../lib/governance-table-state'
import type {
  GovernanceAuditEvent,
  IdentityAuditResult,
} from '../../types'
import { TokenSelect } from '../token-select'
import { AUDIT_REASON_CODES, auditResultVariant } from './audit-utils'
import { IdentityAuditDetailSheet } from './identity-audit-detail-sheet'

/**
 * 身份审计分区页（§11-D）。只读：身份验证失败/降级事件审计。
 *
 * 每行即一个自包含的 `GovernanceAuditEvent`（后端列表接口已返回完整快照；无详情接口，
 * 详情直接从所选行渲染）。列聚焦 D.1 目标：哪个请求出了身份问题（request_id / 时间 /
 * method/path/ip）、使用了哪把 Token（token_id）、命中了哪个 Profile（profile_id）、
 * 声称了哪个 root_app（claimed_root_app_id）、解析出的身份模式与可信等级
 * （identity_mode / identity_assurance）、为什么 UNVERIFIED / REJECTED
 * （result / reason_code）。
 *
 * 筛选全部为后端真实参数（不做假搜索）：global search 精确命中 request_id；
 * token_id 走 TokenSelect；profile_id 走数字输入；result / reason_code 走后端筛选。
 *
 * 严格区分 Profile 配置事实（identity_mode/assurance/caller 等快照）与单次请求运行时
 * 验证事实（result/reason/claimed_root_app），见详情抽屉分组。后端审计事件没有
 * client_verified 字段，前端绝不伪造。
 */
export function IdentityAuditPage() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const tableState = useGovernanceTableState(isMobile ? 10 : 20)
  const [tokenIdFilter, setTokenIdFilter] = useState<number | null>(null)
  const [profileIdFilter, setProfileIdFilter] = useState('')
  const [detailEvent, setDetailEvent] = useState<GovernanceAuditEvent | null>(
    null
  )
  // 手工 Token ID 输入时递增，作为 TokenSelect 的 key 强制重置其展示标签，
  // 避免残留上一个已选中当前 Token 的 name，造成与 tokenIdFilter 不一致。
  const [tokenManualEditSeq, setTokenManualEditSeq] = useState(0)

  const profileIdValue = profileIdFilter ? Number(profileIdFilter) : undefined

  const handleTokenFilterChange = (value: number | null) => {
    setTokenIdFilter(value)
    tableState.onPaginationChange({
      pageIndex: 0,
      pageSize: tableState.pagination.pageSize,
    })
  }

  // 历史/已删除 Token 的手工 token_id 精确查询：直接写同一 tokenIdFilter，
  // 不依赖 TokenSelect 的 getApiKeys/searchApiKeys 是否成功找到该 Token。
  const handleTokenIdInputChange = (raw: string) => {
    const n = raw.trim() === '' ? 0 : Number(raw)
    const next = Number.isInteger(n) && n > 0 ? n : null
    setTokenIdFilter(next)
    if (next != null) {
      setTokenManualEditSeq((s) => s + 1)
    }
    tableState.onPaginationChange({
      pageIndex: 0,
      pageSize: tableState.pagination.pageSize,
    })
  }

  const handleProfileFilterChange = (value: string) => {
    setProfileIdFilter(value)
    tableState.onPaginationChange({
      pageIndex: 0,
      pageSize: tableState.pagination.pageSize,
    })
  }

  // faceted 筛选把选中值存为 string[]，后端 result / reason_code 参数是单值字符串，
  // 这里取 singleSelect 的第一个值，确保转发给后端的是标量而非数组。
  const resultFilterValue = tableState.columnFilters.find(
    (f) => f.id === 'result'
  )?.value
  const resultFilter = Array.isArray(resultFilterValue)
    ? (resultFilterValue[0] as IdentityAuditResult | undefined)
    : (resultFilterValue as IdentityAuditResult | undefined)
  const reasonFilterValue = tableState.columnFilters.find(
    (f) => f.id === 'reason_code'
  )?.value
  const reasonFilter = Array.isArray(reasonFilterValue)
    ? (reasonFilterValue[0] as string | undefined)
    : (reasonFilterValue as string | undefined)

  const columns: ColumnDef<GovernanceAuditEvent>[] = [
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => <TableId value={row.original.id} className='w-[60px]' />,
      size: 80,
    },
    {
      accessorKey: 'result',
      header: t('Result'),
      meta: { mobileBadge: true },
      cell: ({ row }) => (
        <StatusBadge
          label={row.original.result}
          variant={auditResultVariant(row.original.result)}
          copyable={false}
        />
      ),
      size: 120,
    },
    {
      accessorKey: 'reason_code',
      header: t('Reason'),
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.reason_code || '—'}
        </span>
      ),
      size: 180,
    },
    {
      accessorKey: 'request_id',
      header: t('Request ID'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <code className='break-all font-mono text-xs'>{row.original.request_id}</code>
      ),
      size: 200,
    },
    {
      accessorKey: 'token_id',
      header: t('API Key'),
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {row.original.token_id > 0 ? row.original.token_id : '—'}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'profile_id',
      header: t('Profile'),
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {row.original.profile_id > 0 ? row.original.profile_id : '—'}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'identity_mode',
      header: t('Identity Mode'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.identity_mode || '—'}
        </span>
      ),
      size: 110,
    },
    {
      accessorKey: 'identity_assurance',
      header: t('Identity Assurance'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.identity_assurance || '—'}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'claimed_root_app_id',
      header: t('Claimed Root App'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.claimed_root_app_id || '—'}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'caller_id',
      header: t('Caller'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.caller_id || '—'}
        </span>
      ),
      size: 120,
    },
    {
      accessorKey: 'created_at',
      header: t('Time'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[160px] font-mono text-sm'>
          {formatTimestamp(row.original.created_at)}
        </div>
      ),
      size: 180,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => (
        <div className='flex items-center justify-end gap-0.5'>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            onClick={() => setDetailEvent(row.original)}
            aria-label={t('View Details')}
          >
            <Eye className='size-4' />
          </Button>
        </div>
      ),
      meta: { pinned: 'right' as const },
      size: 80,
    },
  ]

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: [
      'ai-governance',
      'identity-audit-events',
      tableState.pagination.pageIndex + 1,
      tableState.pagination.pageSize,
      tableState.globalFilter,
      resultFilter,
      reasonFilter,
      tokenIdFilter,
      profileIdValue,
    ],
    queryFn: () =>
      listIdentityAuditEvents({
        page: tableState.pagination.pageIndex + 1,
        page_size: tableState.pagination.pageSize,
        request_id: tableState.globalFilter || undefined,
        result: resultFilter,
        reason_code: reasonFilter,
        token_id: tokenIdFilter ?? undefined,
        profile_id: profileIdValue,
      }),
    placeholderData: (prev) => prev,
  })

  const { table } = useDataTable({
    data: data?.items ?? [],
    columns,
    columnFilters: tableState.columnFilters,
    globalFilter: tableState.globalFilter,
    pagination: tableState.pagination,
    onPaginationChange: tableState.onPaginationChange,
    onGlobalFilterChange: tableState.onGlobalFilterChange,
    onColumnFiltersChange: tableState.onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange: tableState.ensurePageInRange,
  })

  return (
    <>
      <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
        <p className='max-w-2xl text-sm text-muted-foreground'>
          {t('Identity audit records which request had an identity problem, which token and profile it hit, what the client claimed, and whether it was downgraded or rejected.')}
        </p>
      </div>

      <div className='mb-3 flex flex-wrap items-center gap-2'>
        <TokenSelect
          key={tokenManualEditSeq}
          value={tokenIdFilter}
          onChange={handleTokenFilterChange}
          showTokenId
          className='w-[260px]'
        />
        <Input
          type='number'
          min={1}
          value={tokenIdFilter != null ? String(tokenIdFilter) : ''}
          onChange={(e) => handleTokenIdInputChange(e.target.value)}
          placeholder={t('Token ID')}
          aria-label={t('Token ID')}
          className='w-[140px]'
        />
        {tokenIdFilter != null && (
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => handleTokenFilterChange(null)}
          >
            {t('Clear')}
          </Button>
        )}
        <Input
          type='number'
          min={1}
          value={profileIdFilter}
          onChange={(e) => handleProfileFilterChange(e.target.value)}
          placeholder={t('Profile ID')}
          aria-label={t('Profile ID')}
          className='w-[140px]'
        />
        {profileIdFilter !== '' && (
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => handleProfileFilterChange('')}
          >
            {t('Clear')}
          </Button>
        )}
      </div>

      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        isError={isError}
        errorTitle={t('Oops! Something went wrong')}
        errorDescription={t('Failed to load')}
        onErrorRetry={() => void refetch()}
        emptyTitle={t('No identity audit events found')}
        emptyDescription={t('No identity audit events match the current search and filters.')}
        skeletonKeyPrefix='identity-audit-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by request ID...'),
          searchDebounceMs: 500,
          filters: [
            {
              columnId: 'result',
              title: t('Result'),
              options: [
                { label: 'UNVERIFIED', value: 'UNVERIFIED' },
                { label: 'REJECTED', value: 'REJECTED' },
              ],
              singleSelect: true,
            },
            {
              columnId: 'reason_code',
              title: t('Reason'),
              options: AUDIT_REASON_CODES.map((code) => ({
                label: code,
                value: code,
              })),
              singleSelect: true,
            },
          ],
        }}
      />

      <IdentityAuditDetailSheet
        event={detailEvent}
        open={detailEvent != null}
        onOpenChange={(v) => {
          if (!v) setDetailEvent(null)
        }}
      />
    </>
  )
}
