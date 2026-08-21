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
import { Circle, CircleOff } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import { useMediaQuery } from '@/hooks'
import { formatTimestamp } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import { listIdentityProfiles, updateIdentityProfile } from '../../api'
import { parseEnabledFilter } from '../../lib/enabled-filter'
import { useGovernanceTableState } from '../../lib/governance-table-state'
import type {
  CredentialRiskLevel,
  GovernanceIdentityProfileDetail,
  IdentityMode,
} from '../../types'
import { EnabledBadge } from '../enabled-badge'
import { TokenSelect } from '../token-select'

function modeVariant(mode: IdentityMode): 'neutral' | 'info' | 'warning' {
  if (mode === 'STATIC') return 'neutral'
  if (mode === 'DYNAMIC') return 'info'
  return 'warning'
}

function riskVariant(level: CredentialRiskLevel): 'success' | 'warning' | 'danger' {
  if (level === 'LOWER_RISK') return 'success'
  if (level === 'MEDIUM_RISK') return 'warning'
  return 'danger'
}

/**
 * API Key 身份（Identity Profile）列表分区页（§11-C §C.1）。
 *
 * 每行即聚合详情 `GovernanceIdentityProfileDetail`（后端列表接口已返回完整聚合）。
 * 列：token / identity_mode / attribution_target / identity_assurance /
 * principal / purpose / environment / enabled / risk / updated_at。
 *
 * 筛选语义（真实能力，非假搜索）：
 * - keyword 只命中后端 caller_id / caller_name（不搜 token_name）；
 * - identity_mode / enabled 走后端参数；
 * - token_id 走独立 Token Selector，作为后端 `token_id` 参数（不得做当前页假搜索）。
 *
 * C.1 行操作仅启用/停用（Update.enabled）；编辑/详情在 C.2/C.3 接入。
 */
export function IdentityProfilesPage() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const tableState = useGovernanceTableState(isMobile ? 10 : 20)
  const [tokenIdFilter, setTokenIdFilter] = useState<number | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  // 停用/启用确认弹窗状态（C.1 仅启停，编辑/详情后续子批接入）
  const [toggleRow, setToggleRow] = useState<GovernanceIdentityProfileDetail | null>(null)
  const [isToggling, setIsToggling] = useState(false)

  const handleTokenFilterChange = (value: number | null) => {
    setTokenIdFilter(value)
    tableState.onPaginationChange({
      pageIndex: 0,
      pageSize: tableState.pagination.pageSize,
    })
  }

  const columns: ColumnDef<GovernanceIdentityProfileDetail>[] = [
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <TableId value={row.original.profile.id} className='w-[60px]' />
      ),
      size: 80,
    },
    {
      accessorKey: 'token_name',
      header: t('API Key'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <div className='flex flex-col'>
          <span className='font-medium'>{row.original.token.token_name}</span>
          <span className='text-xs text-muted-foreground'>
            {t('Token ID')}: {row.original.token.token_id}
          </span>
        </div>
      ),
      size: 200,
    },
    {
      accessorKey: 'identity_mode',
      header: t('Identity Mode'),
      cell: ({ row }) => (
        <StatusBadge
          label={row.original.profile.identity_mode}
          variant={modeVariant(row.original.profile.identity_mode)}
          copyable={false}
        />
      ),
      filterFn: (row, _id, value: unknown) =>
        row.original.profile.identity_mode === value,
      size: 120,
    },
    {
      accessorKey: 'attribution_target_type',
      header: t('Attribution Target'),
      cell: ({ row }) => (
        <span className='text-sm text-muted-foreground'>
          {row.original.profile.attribution_target_type}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'identity_assurance',
      header: t('Identity Assurance'),
      cell: ({ row }) => (
        <StatusBadge
          label={row.original.profile.identity_assurance}
          variant='neutral'
          copyable={false}
        />
      ),
      size: 180,
    },
    {
      accessorKey: 'principal',
      header: t('Principal'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {row.original.principal?.principal_name ?? '—'}
        </span>
      ),
      size: 160,
    },
    {
      accessorKey: 'purpose',
      header: t('Credential Purpose'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {row.original.purpose?.credential_purpose_name ?? '—'}
        </span>
      ),
      size: 180,
    },
    {
      accessorKey: 'environment',
      header: t('Environment'),
      cell: ({ row }) => (
        <span className='text-sm'>{row.original.profile.environment}</span>
      ),
      size: 110,
    },
    {
      accessorKey: 'enabled',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => <EnabledBadge enabled={row.original.profile.enabled} />,
      filterFn: (row, _id, value: unknown) => {
        const enabled = row.original.profile.enabled
        return value === 'true' ? enabled : !enabled
      },
      size: 120,
    },
    {
      accessorKey: 'risk',
      header: t('Risk'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <StatusBadge
          label={row.original.risk.risk_level}
          variant={riskVariant(row.original.risk.risk_level)}
          copyable={false}
        />
      ),
      size: 130,
    },
    {
      accessorKey: 'updated_at',
      header: t('Updated'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[160px] font-mono text-sm'>
          {formatTimestamp(row.original.profile.updated_at)}
        </div>
      ),
      size: 180,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => {
        const detail = row.original
        const enabled = detail.profile.enabled
        return (
          <div className='flex items-center justify-end gap-0.5'>
            <Button
              type='button'
              variant='ghost'
              size='icon'
              onClick={() => setToggleRow(detail)}
              aria-label={t(enabled ? 'Disable' : 'Enable')}
            >
              {enabled ? (
                <CircleOff className='size-4 text-muted-foreground' />
              ) : (
                <Circle className='size-4 text-muted-foreground' />
              )}
            </Button>
          </div>
        )
      },
      meta: { pinned: 'right' as const },
      size: 60,
    },
  ]

  const modeFilter = tableState.columnFilters.find(
    (f) => f.id === 'identity_mode'
  )?.value as string | undefined
  const enabledFilter = tableState.columnFilters.find(
    (f) => f.id === 'enabled'
  )?.value as string | undefined
  const enabledValue = parseEnabledFilter(enabledFilter)

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: [
      'ai-governance',
      'identity-profiles',
      tableState.pagination.pageIndex + 1,
      tableState.pagination.pageSize,
      tableState.globalFilter,
      modeFilter,
      enabledValue,
      tokenIdFilter,
      refreshTrigger,
    ],
    queryFn: () =>
      listIdentityProfiles({
        page: tableState.pagination.pageIndex + 1,
        page_size: tableState.pagination.pageSize,
        keyword: tableState.globalFilter || undefined,
        identity_mode: modeFilter as IdentityMode | undefined,
        enabled: enabledValue,
        token_id: tokenIdFilter ?? undefined,
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

  const handleToggle = async () => {
    if (!toggleRow) return
    setIsToggling(true)
    try {
      const target = !toggleRow.profile.enabled
      await updateIdentityProfile(toggleRow.profile.id, { enabled: target })
      toast.success(
        t(target ? 'Identity profile enabled' : 'Identity profile disabled')
      )
      setToggleRow(null)
      triggerRefresh()
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsToggling(false)
    }
  }

  return (
    <>
      <div className='mb-4'>
        <p className='max-w-2xl text-sm text-muted-foreground'>
          {t('API Key identity binds a NewAPI token to a governed identity profile. Search matches the caller ID or caller name.')}
        </p>
      </div>

      <div className='mb-3 flex flex-wrap items-center gap-2'>
        <TokenSelect
          value={tokenIdFilter}
          onChange={handleTokenFilterChange}
          className='w-[260px]'
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
        emptyTitle={t('No identity profiles found')}
        emptyDescription={t('No identity profiles match the current search and filters.')}
        skeletonKeyPrefix='identity-profiles-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by caller ID or caller name...'),
          searchDebounceMs: 500,
          filters: [
            {
              columnId: 'identity_mode',
              title: t('Identity Mode'),
              options: [
                { label: 'STATIC', value: 'STATIC' },
                { label: 'DYNAMIC', value: 'DYNAMIC' },
                { label: 'HYBRID', value: 'HYBRID' },
              ],
              singleSelect: true,
            },
            {
              columnId: 'enabled',
              title: t('Status'),
              options: [
                { label: t('Enabled'), value: 'true' },
                { label: t('Disabled'), value: 'false' },
              ],
              singleSelect: true,
            },
          ],
        }}
      />

      <ConfirmDialog
        open={toggleRow != null}
        onOpenChange={(v) => {
          if (!v) setToggleRow(null)
        }}
        title={t(
          toggleRow?.profile.enabled
            ? 'Disable this identity profile?'
            : 'Enable this identity profile?'
        )}
        desc={
          toggleRow?.profile.enabled
            ? t('This profile will no longer be used for identity attribution. Existing references are preserved.')
            : t('This profile will be used for identity attribution.')
        }
        confirmText={t(
          toggleRow?.profile.enabled ? 'Disable' : 'Enable'
        )}
        destructive={toggleRow?.profile.enabled ?? false}
        handleConfirm={handleToggle}
        isLoading={isToggling}
      />
    </>
  )
}
