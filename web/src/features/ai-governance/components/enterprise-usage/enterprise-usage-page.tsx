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
import { AlertTriangle, DatabaseZap } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect } from '@/components/ui/native-select'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { useMediaQuery } from '@/hooks'
import { handleServerError } from '@/lib/handle-server-error'
import {
  formatLogQuota,
  formatNumber,
  formatTimestamp,
  formatTokens,
} from '@/lib/format'

import {
  listEnterpriseUsage,
  listEnterpriseUsageAnomalies,
  rebuildEnterpriseUsage,
} from '../../api'
import { useGovernanceTableState } from '../../lib/governance-table-state'
import type {
  EnterpriseUsageAnomaly,
  EnterpriseUsageFilter,
  EnterpriseUsageRow,
  UsageIdentityAssurance,
} from '../../types'
import {
  BusinessDomainSelect,
  OwnerTeamSelect,
  UsageTeamSelect,
} from '../master-data-selects'
import {
  UsagePrincipalSelect,
  UsageProfileSelect,
  UsagePurposeSelect,
} from './enterprise-usage-selects'

const ASSURANCE_OPTIONS: readonly UsageIdentityAssurance[] = [
  'HYBRID_VERIFIED_CONTEXT',
  'SIGNED_CONTEXT',
  'CREDENTIAL_ONLY',
  'UNVERIFIED',
]

type BadgeVariant = 'default' | 'secondary' | 'warning' | 'outline'

/**
 * 用量投影中的身份可信等级徽标配色。
 * 仅作可视化提示，不在此重新推导强身份 Caller/App 归因。
 */
function usageAssuranceVariant(a: UsageIdentityAssurance): BadgeVariant {
  switch (a) {
    case 'HYBRID_VERIFIED_CONTEXT':
      return 'default'
    case 'SIGNED_CONTEXT':
      return 'secondary'
    case 'CREDENTIAL_ONLY':
      return 'warning'
    default:
      return 'outline'
  }
}

/** client_verified 徽标：true=Verified(success)、false=Unverified(neutral)。 */
function verifiedVariant(verified: boolean): 'success' | 'neutral' {
  return verified ? 'success' : 'neutral'
}

function toUnixSec(date?: Date): number | undefined {
  return date ? Math.floor(date.getTime() / 1000) : undefined
}

/**
 * 企业用量分区页（§11-E）。只读统计 + 异常 + Root 重建。
 *
 * 页面把 `AIUsageHourly` 明确当作主库中的查询投影（非计费事实源）：NewAPI 原始
 * 消费/计费 Log 仍是事实源。本页只读展示投影与异常，不修改既有统计语义。
 *
 * client_verified 语义严格保持 §12 冻结定义：只有 DYNAMIC/HYBRID 且
 * client_verified=true 的行才代表可核验的 Caller/App 归因；本页只展示该字段，
 * 不因某行带 caller/root_app 字符串就把它当作可信归因。
 */
export function EnterpriseUsagePage() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  // ---- 统计筛选（全部为后端真实参数） ----
  const [bucketRange, setBucketRange] = useState<{
    start?: Date
    end?: Date
  }>({})
  const [profileId, setProfileId] = useState<number | null>(null)
  const [principalId, setPrincipalId] = useState<number | null>(null)
  const [purposeId, setPurposeId] = useState<number | null>(null)
  const [usageDomainId, setUsageDomainId] = useState<number | null>(null)
  const [usageTeamId, setUsageTeamId] = useState<number | null>(null)
  const [appDomainId, setAppDomainId] = useState<number | null>(null)
  const [ownerTeamId, setOwnerTeamId] = useState<number | null>(null)
  const [callerKey, setCallerKey] = useState('')
  const [rootAppCode, setRootAppCode] = useState('')
  const [modelName, setModelName] = useState('')
  const [assurance, setAssurance] = useState<UsageIdentityAssurance | ''>('')

  const statsState = useGovernanceTableState(isMobile ? 10 : 20)
  const anomalyState = useGovernanceTableState(isMobile ? 10 : 20)

  const resetStatsPage = () =>
    statsState.onPaginationChange({
      pageIndex: 0,
      pageSize: statsState.pagination.pageSize,
    })

  const filter: EnterpriseUsageFilter = {
    bucket_start: toUnixSec(bucketRange.start),
    bucket_end: toUnixSec(bucketRange.end),
    profile_id: profileId ?? undefined,
    principal_id: principalId ?? undefined,
    credential_purpose_id: purposeId ?? undefined,
    usage_business_domain_id: usageDomainId ?? undefined,
    usage_team_id: usageTeamId ?? undefined,
    app_business_domain_id: appDomainId ?? undefined,
    owner_team_id: ownerTeamId ?? undefined,
    caller_key: callerKey.trim() || undefined,
    root_app_code: rootAppCode.trim() || undefined,
    model_name: modelName.trim() || undefined,
    identity_assurance: assurance || undefined,
  }

  const statsQuery = useQuery({
    queryKey: ['ai-governance', 'enterprise-usage-stats', filter],
    queryFn: () => listEnterpriseUsage(filter),
    placeholderData: (prev) => prev,
  })

  const columns: ColumnDef<EnterpriseUsageRow>[] = [
    {
      accessorKey: 'bucket_time',
      header: t('Time'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[150px] font-mono text-sm'>
          {formatTimestamp(row.original.bucket_time)}
        </div>
      ),
      size: 170,
    },
    {
      accessorKey: 'caller_key',
      header: t('Caller'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.caller_key || '—'}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'root_app_code',
      header: t('Root App'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.root_app_code || '—'}
        </span>
      ),
      size: 130,
    },
    {
      accessorKey: 'identity_assurance',
      header: t('Identity Assurance'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <Badge variant={usageAssuranceVariant(row.original.identity_assurance)}>
          {row.original.identity_assurance || '—'}
        </Badge>
      ),
      size: 170,
    },
    {
      accessorKey: 'client_verified',
      header: t('Client Verified'),
      cell: ({ row }) => (
        <Badge variant={verifiedVariant(row.original.client_verified) === 'success' ? 'default' : 'outline'}>
          {row.original.client_verified ? t('Verified') : t('Unverified')}
        </Badge>
      ),
      size: 120,
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.model_name || '—'}
        </span>
      ),
      size: 160,
    },
    {
      accessorKey: 'request_count',
      header: t('Requests'),
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatNumber(row.original.request_count)}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'success_count',
      header: t('Success'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatNumber(row.original.success_count)}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'error_count',
      header: t('Errors'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatNumber(row.original.error_count)}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'input_tokens',
      header: t('Input Tokens'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatTokens(row.original.input_tokens)}
        </span>
      ),
      size: 110,
    },
    {
      accessorKey: 'output_tokens',
      header: t('Output Tokens'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatTokens(row.original.output_tokens)}
        </span>
      ),
      size: 110,
    },
    {
      accessorKey: 'total_tokens',
      header: t('Total Tokens'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatTokens(row.original.total_tokens)}
        </span>
      ),
      size: 110,
    },
    {
      accessorKey: 'quota_net',
      header: t('Net Quota'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatLogQuota(row.original.quota_net)}
        </span>
      ),
      size: 110,
    },
  ]

  const { table: statsTable } = useDataTable({
    data: statsQuery.data ?? [],
    columns,
    columnFilters: statsState.columnFilters,
    globalFilter: statsState.globalFilter,
    pagination: statsState.pagination,
    onPaginationChange: statsState.onPaginationChange,
    onGlobalFilterChange: statsState.onGlobalFilterChange,
    onColumnFiltersChange: statsState.onColumnFiltersChange,
    ensurePageInRange: statsState.ensurePageInRange,
  })

  // ---- 异常 ----
  const [anomalyRange, setAnomalyRange] = useState<{
    start?: Date
    end?: Date
  }>({})

  // 后端 anomalies 接口要求真实的 Unix 秒起止；未选范围时默认最近 7 天。
  const nowSec = Math.floor(Date.now() / 1000)
  const anomalyStart = toUnixSec(anomalyRange.start) ?? nowSec - 7 * 24 * 3600
  const anomalyEnd = toUnixSec(anomalyRange.end) ?? nowSec

  const anomalyQuery = useQuery({
    queryKey: ['ai-governance', 'enterprise-usage-anomalies', anomalyStart, anomalyEnd],
    queryFn: () => listEnterpriseUsageAnomalies(anomalyStart, anomalyEnd),
    placeholderData: (prev) => prev,
  })

  const anomalyColumns: ColumnDef<EnterpriseUsageAnomaly>[] = [
    {
      accessorKey: 'bucket_time',
      header: t('Time'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <div className='min-w-[150px] font-mono text-sm'>
          {formatTimestamp(row.original.bucket_time)}
        </div>
      ),
      size: 170,
    },
    {
      accessorKey: 'metric',
      header: t('Metric'),
      cell: ({ row }) => (
        <span className='font-mono text-sm'>{row.original.metric}</span>
      ),
      size: 100,
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='break-all font-mono text-xs'>
          {row.original.model_name || '—'}
        </span>
      ),
      size: 150,
    },
    {
      accessorKey: 'identity_assurance',
      header: t('Identity Assurance'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <Badge variant={usageAssuranceVariant(row.original.identity_assurance)}>
          {row.original.identity_assurance || '—'}
        </Badge>
      ),
      size: 170,
    },
    {
      accessorKey: 'current',
      header: t('Current'),
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatNumber(row.original.current)}
        </span>
      ),
      size: 100,
    },
    {
      accessorKey: 'baseline',
      header: t('Baseline'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatNumber(row.original.baseline)}
        </span>
      ),
      size: 100,
    },
    {
      accessorKey: 'threshold',
      header: t('Threshold'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {formatNumber(row.original.threshold)}
        </span>
      ),
      size: 100,
    },
    {
      accessorKey: 'profile_id',
      header: t('Profile'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {row.original.profile_id > 0 ? row.original.profile_id : '—'}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'principal_id',
      header: t('Principal'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {row.original.principal_id > 0 ? row.original.principal_id : '—'}
        </span>
      ),
      size: 90,
    },
    {
      accessorKey: 'credential_purpose_id',
      header: t('Purpose'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm'>
          {row.original.credential_purpose_id > 0
            ? row.original.credential_purpose_id
            : '—'}
        </span>
      ),
      size: 90,
    },
  ]

  const { table: anomalyTable } = useDataTable({
    data: anomalyQuery.data ?? [],
    columns: anomalyColumns,
    columnFilters: anomalyState.columnFilters,
    globalFilter: anomalyState.globalFilter,
    pagination: anomalyState.pagination,
    onPaginationChange: anomalyState.onPaginationChange,
    onGlobalFilterChange: anomalyState.onGlobalFilterChange,
    onColumnFiltersChange: anomalyState.onColumnFiltersChange,
    ensurePageInRange: anomalyState.ensurePageInRange,
  })

  // ---- 重建（Root 管理操作） ----
  const [rebuildOpen, setRebuildOpen] = useState(false)
  const [rebuildRange, setRebuildRange] = useState<{
    start?: Date
    end?: Date
  }>({})
  const [rebuilding, setRebuilding] = useState(false)
  const [processedLogs, setProcessedLogs] = useState<number | null>(null)

  const rebuildStart = toUnixSec(rebuildRange.start)
  const rebuildEnd = toUnixSec(rebuildRange.end)
  const rebuildRangeValid =
    rebuildStart != null &&
    rebuildEnd != null &&
    rebuildEnd > rebuildStart

  const handleRebuild = async () => {
    if (!rebuildRangeValid || rebuildStart == null || rebuildEnd == null) return
    setRebuilding(true)
    try {
      const res = await rebuildEnterpriseUsage(rebuildStart, rebuildEnd)
      setProcessedLogs(res.processed_logs)
      toast.success(
        t('Usage projection rebuilt. {{count}} logs processed.', {
          count: res.processed_logs,
        })
      )
      setRebuildOpen(false)
      void statsQuery.refetch()
      void anomalyQuery.refetch()
    } catch (error) {
      handleServerError(error)
    } finally {
      setRebuilding(false)
    }
  }

  const hasActiveStatsFilter =
    filter.bucket_start != null ||
    filter.bucket_end != null ||
    filter.profile_id != null ||
    filter.principal_id != null ||
    filter.credential_purpose_id != null ||
    filter.usage_business_domain_id != null ||
    filter.usage_team_id != null ||
    filter.app_business_domain_id != null ||
    filter.owner_team_id != null ||
    filter.caller_key != null ||
    filter.root_app_code != null ||
    filter.model_name != null ||
    filter.identity_assurance != null

  const resetStatsFilters = () => {
    setBucketRange({})
    setProfileId(null)
    setPrincipalId(null)
    setPurposeId(null)
    setUsageDomainId(null)
    setUsageTeamId(null)
    setAppDomainId(null)
    setOwnerTeamId(null)
    setCallerKey('')
    setRootAppCode('')
    setModelName('')
    setAssurance('')
    resetStatsPage()
  }

  const fieldLabel = (text: string) => (
    <span className='mb-1 block text-xs font-medium text-muted-foreground'>
      {text}
    </span>
  )

  return (
    <>
      <div className='mb-4 flex flex-wrap items-start justify-between gap-3'>
        <p className='max-w-2xl text-sm text-muted-foreground'>
          {t('Enterprise usage projects AI consumption by domain, team, person, purpose, app and caller. It is a read-only projection of the NewAPI consumption log, which remains the billing source of truth. Only DYNAMIC and HYBRID rows with client_verified equal true count as verified caller and app attribution.')}
        </p>
      </div>

      {/* 统计筛选 */}
      <section aria-label={t('Usage statistics filters')} className='mb-6'>
        <div className='mb-3 flex flex-wrap items-center justify-between gap-3'>
          <h2 className='flex items-center gap-2 text-base font-semibold'>
            <DatabaseZap className='size-4 text-muted-foreground' />
            {t('Usage Statistics')}
          </h2>
          {hasActiveStatsFilter && (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={resetStatsFilters}
            >
              {t('Clear All Filters')}
            </Button>
          )}
        </div>

        <div className='mb-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4'>
          <div>
            {fieldLabel(t('Bucket Time Range'))}
            <CompactDateTimeRangePicker
              start={bucketRange.start}
              end={bucketRange.end}
              onChange={(range) => {
                setBucketRange(range)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Identity Profile'))}
            <UsageProfileSelect
              value={profileId}
              onChange={(v) => {
                setProfileId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Principal'))}
            <UsagePrincipalSelect
              value={principalId}
              onChange={(v) => {
                setPrincipalId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Credential Purpose'))}
            <UsagePurposeSelect
              value={purposeId}
              onChange={(v) => {
                setPurposeId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Usage Business Domain'))}
            <BusinessDomainSelect
              value={usageDomainId}
              onChange={(v) => {
                setUsageDomainId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Usage Team'))}
            <UsageTeamSelect
              value={usageTeamId}
              onChange={(v) => {
                setUsageTeamId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('App Business Domain'))}
            <BusinessDomainSelect
              value={appDomainId}
              onChange={(v) => {
                setAppDomainId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Owner Team'))}
            <OwnerTeamSelect
              value={ownerTeamId}
              onChange={(v) => {
                setOwnerTeamId(v)
                resetStatsPage()
              }}
              className='w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Caller Key'))}
            <Input
              value={callerKey}
              onChange={(e) => {
                setCallerKey(e.target.value)
                resetStatsPage()
              }}
              placeholder={t('Caller key')}
              aria-label={t('Caller Key')}
              className='h-8 w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Root App Code'))}
            <Input
              value={rootAppCode}
              onChange={(e) => {
                setRootAppCode(e.target.value)
                resetStatsPage()
              }}
              placeholder={t('Root app code')}
              aria-label={t('Root App Code')}
              className='h-8 w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Model Name'))}
            <Input
              value={modelName}
              onChange={(e) => {
                setModelName(e.target.value)
                resetStatsPage()
              }}
              placeholder={t('Model name')}
              aria-label={t('Model Name')}
              className='h-8 w-full'
            />
          </div>
          <div>
            {fieldLabel(t('Identity Assurance'))}
            <NativeSelect
              value={assurance}
              onChange={(e) => {
                setAssurance(e.target.value as UsageIdentityAssurance | '')
                resetStatsPage()
              }}
              aria-label={t('Identity Assurance')}
              className='h-8 w-full'
            >
              <option value=''>{t('All')}</option>
              {ASSURANCE_OPTIONS.map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </NativeSelect>
          </div>
        </div>

        <DataTablePage
          table={statsTable}
          columns={columns}
          isLoading={statsQuery.isLoading}
          isFetching={statsQuery.isFetching}
          isError={statsQuery.isError}
          errorTitle={t('Oops! Something went wrong')}
          errorDescription={t('Failed to load')}
          onErrorRetry={() => void statsQuery.refetch()}
          emptyTitle={t('No usage statistics found')}
          emptyDescription={t('No usage projection rows match the current filters.')}
          skeletonKeyPrefix='enterprise-usage-stats-skeleton'
          applyHeaderSize
          toolbarProps={null}
        />
      </section>

      {/* 异常 */}
      <section aria-label={t('Usage anomalies')} className='mb-6'>
        <div className='mb-3 flex flex-wrap items-center justify-between gap-3'>
          <h2 className='flex items-center gap-2 text-base font-semibold'>
            <AlertTriangle className='size-4 text-muted-foreground' />
            {t('Usage Anomalies')}
          </h2>
        </div>
        <div className='mb-3 max-w-sm'>
          {fieldLabel(t('Anomaly Detection Range'))}
          <CompactDateTimeRangePicker
            start={anomalyRange.start}
            end={anomalyRange.end}
            onChange={(range) => setAnomalyRange(range)}
            className='w-full'
          />
        </div>
        <DataTablePage
          table={anomalyTable}
          columns={anomalyColumns}
          isLoading={anomalyQuery.isLoading}
          isFetching={anomalyQuery.isFetching}
          isError={anomalyQuery.isError}
          errorTitle={t('Oops! Something went wrong')}
          errorDescription={t('Failed to load')}
          onErrorRetry={() => void anomalyQuery.refetch()}
          emptyTitle={t('No usage anomalies found')}
          emptyDescription={t('No usage anomalies were detected in the selected range.')}
          skeletonKeyPrefix='enterprise-usage-anomalies-skeleton'
          applyHeaderSize
          toolbarProps={null}
        />
      </section>

      {/* 重建（Root 管理操作） */}
      <section aria-label={t('Rebuild usage projection')}>
        <div className='mb-3 flex flex-wrap items-center justify-between gap-3'>
          <h2 className='flex items-center gap-2 text-base font-semibold'>
            <DatabaseZap className='size-4 text-muted-foreground' />
            {t('Rebuild Projection')}
          </h2>
          <Badge variant='warning'>{t('Root only')}</Badge>
        </div>
        <p className='mb-3 max-w-2xl text-sm text-muted-foreground'>
          {t('Rebuild reprocesses the NewAPI consumption log within the selected Unix time range and recomputes the hourly usage projection. This is a management operation and must be confirmed before it runs.')}
        </p>
        <div className='mb-3 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3'>
          <div>
            {fieldLabel(t('Rebuild Time Range'))}
            <CompactDateTimeRangePicker
              start={rebuildRange.start}
              end={rebuildRange.end}
              onChange={(range) => {
                setRebuildRange(range)
                setProcessedLogs(null)
              }}
              className='w-full'
            />
          </div>
          <div className='flex items-end'>
            <div className='w-full'>
              {fieldLabel(t('Unix Range (seconds)'))}
              <div className='break-all font-mono text-xs text-muted-foreground'>
                {rebuildStart != null ? rebuildStart : '—'} ~{' '}
                {rebuildEnd != null ? rebuildEnd : '—'}
              </div>
            </div>
          </div>
          <div className='flex items-end'>
            <Button
              type='button'
              variant='destructive'
              disabled={!rebuildRangeValid}
              onClick={() => setRebuildOpen(true)}
            >
              {t('Rebuild Projection')}
            </Button>
          </div>
        </div>

        {processedLogs != null && (
          <p className='text-sm text-muted-foreground'>
            {t('Last rebuild processed {{count}} logs.', {
              count: processedLogs,
            })}
          </p>
        )}
      </section>

      <ConfirmDialog
        open={rebuildOpen}
        onOpenChange={setRebuildOpen}
        title={t('Rebuild Usage Projection?')}
        desc={t('This reprocesses the NewAPI consumption log from {{start}} to {{end}} (Unix seconds) and recomputes the hourly projection. This cannot be undone. Continue?', {
          start: rebuildStart ?? '—',
          end: rebuildEnd ?? '—',
        })}
        destructive
        confirmText={t('Rebuild')}
        handleConfirm={() => void handleRebuild()}
        isLoading={rebuilding}
        disabled={!rebuildRangeValid}
      />
    </>
  )
}
