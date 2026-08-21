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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  DataTablePage,
  useDataTable,
} from '@/components/data-table'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { useMediaQuery } from '@/hooks'
import useDialogState from '@/hooks/use-dialog'
import { formatTimestamp } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createPrincipal,
  listBusinessDomains,
  listPrincipals,
  listUsageTeams,
  updatePrincipal,
} from '../../api'
import { getGovernanceCodeSchema } from '../../lib/code-schema'
import { parseEnabledFilter } from '../../lib/enabled-filter'
import { useGovernanceTableState } from '../../lib/governance-table-state'
import { useMasterDataReference } from '../../lib/master-data-reference'
import type { GovernancePrincipal } from '../../types'
import { EnabledBadge } from '../enabled-badge'
import {
  BusinessDomainSelect,
  UsageTeamSelect,
} from '../master-data-selects'
import { MasterDataRowActions } from '../master-data-row-actions'

const PRINCIPAL_CODE_HELP =
  'Starts with a lowercase letter; lowercase letters, numbers, dots, underscores or hyphens; 2-64 characters. Cannot be changed after creation.'

type FormValues = {
  principal_code: string
  principal_name: string
  business_domain_id: number | null
  usage_team_id: number | null
}

function requiredId(t: (key: string) => string, message: string) {
  return z.number().nullable().refine((v) => v != null, t(message))
}

/**
 * 使用主体分区页（§11-B §5，最复杂）。
 *
 * principal_type 固定 PERSON，不允许选择；创建/编辑用 Business Domain Selector +
 * Usage Team Selector（分页搜索加载）；编辑时 principal_code 只读；列含
 * business_domain / usage_team（引用数据解析名称）；筛选 keyword/enabled/
 * business_domain_id/usage_team_id。
 */
export function PrincipalsPage() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const [open, setOpen] = useDialogState<'create' | 'update' | null>(null)
  const [currentRow, setCurrentRow] = useState<GovernancePrincipal | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  const tableState = useGovernanceTableState(isMobile ? 10 : 20)

  const domainRef = useMasterDataReference({
    queryKey: ['ai-governance', 'business-domains'],
    fetchPage: listBusinessDomains,
    itemToValue: (item) => item.id,
    itemToLabel: (item) => item.domain_name,
  })
  const teamRef = useMasterDataReference({
    queryKey: ['ai-governance', 'usage-teams'],
    fetchPage: listUsageTeams,
    itemToValue: (item) => item.id,
    itemToLabel: (item) => item.team_name,
  })

  const columns: ColumnDef<GovernancePrincipal>[] = [
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <TableId value={row.original.id} className='w-[60px]' />
      ),
      size: 80,
    },
    {
      accessorKey: 'principal_code',
      header: t('Principal Code'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <code className='text-sm font-medium'>{row.original.principal_code}</code>
      ),
      size: 180,
    },
    {
      accessorKey: 'principal_name',
      header: t('Principal Name'),
      cell: ({ row }) => (
        <span className='font-medium'>{row.original.principal_name}</span>
      ),
      size: 200,
    },
    {
      accessorKey: 'business_domain_id',
      header: t('Business Domain'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {domainRef.data?.byId.get(row.original.business_domain_id) ??
            String(row.original.business_domain_id)}
        </span>
      ),
      filterFn: (row, _id, value: unknown) =>
        String(row.original.business_domain_id) === value,
      size: 160,
    },
    {
      accessorKey: 'usage_team_id',
      header: t('Usage Team'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {teamRef.data?.byId.get(row.original.usage_team_id) ??
            String(row.original.usage_team_id)}
        </span>
      ),
      filterFn: (row, _id, value: unknown) =>
        String(row.original.usage_team_id) === value,
      size: 160,
    },
    {
      accessorKey: 'principal_type',
      header: t('Type'),
      cell: () => <span className='text-sm text-muted-foreground'>{t('Person')}</span>,
      size: 100,
    },
    {
      accessorKey: 'enabled',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => <EnabledBadge enabled={row.original.enabled} />,
      filterFn: (row, _id, value: unknown) => {
        const enabled = row.original.enabled
        return value === 'true' ? enabled : !enabled
      },
      size: 120,
    },
    {
      accessorKey: 'updated_at',
      header: t('Updated'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[160px] font-mono text-sm'>
          {formatTimestamp(row.original.updated_at)}
        </div>
      ),
      size: 180,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => {
        const entity = row.original
        return (
          <MasterDataRowActions
            isEnabled={entity.enabled}
            onEdit={() => openUpdate(entity)}
            onToggle={async (enabled) => {
              try {
                await updatePrincipal(entity.id, {
                  principal_name: entity.principal_name,
                  business_domain_id: entity.business_domain_id,
                  usage_team_id: entity.usage_team_id,
                  enabled,
                })
                toast.success(
                  t(enabled ? 'Principal enabled' : 'Principal disabled')
                )
                triggerRefresh()
              } catch (error) {
                handleServerError(error)
                throw error
              }
            }}
            enableDesc={t('This principal will be available for assignment.')}
            disableDesc={t('This principal will no longer be selectable. Existing references are preserved.')}
          />
        )
      },
      meta: { pinned: 'right' as const },
      size: 120,
    },
  ]

  const domainFilter = tableState.columnFilters.find(
    (f) => f.id === 'business_domain_id'
  )?.value as string | undefined
  const teamFilter = tableState.columnFilters.find(
    (f) => f.id === 'usage_team_id'
  )?.value as string | undefined
  const enabledFilter = tableState.columnFilters.find(
    (f) => f.id === 'enabled'
  )?.value as string | undefined
  const enabledValue = parseEnabledFilter(enabledFilter)

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'ai-governance',
      'principals',
      tableState.pagination.pageIndex + 1,
      tableState.pagination.pageSize,
      tableState.globalFilter,
      domainFilter,
      teamFilter,
      enabledValue,
      refreshTrigger,
    ],
    queryFn: () =>
      listPrincipals({
        page: tableState.pagination.pageIndex + 1,
        page_size: tableState.pagination.pageSize,
        keyword: tableState.globalFilter || undefined,
        business_domain_id: domainFilter ? Number(domainFilter) : undefined,
        usage_team_id: teamFilter ? Number(teamFilter) : undefined,
        enabled: enabledValue,
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

  const schema = z.object({
    principal_code: getGovernanceCodeSchema(t),
    principal_name: z
      .string()
      .min(1, t('Name is required'))
      .max(200, t('Name must be at most 200 characters')),
    business_domain_id: requiredId(t, 'Please select a business domain'),
    usage_team_id: requiredId(t, 'Please select a usage team'),
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      principal_code: '',
      principal_name: '',
      business_domain_id: null,
      usage_team_id: null,
    },
  })

  const isUpdate = open === 'update'
  const isOpen = open !== null
  const [isSubmitting, setIsSubmitting] = useState(false)

  const openCreate = () => {
    setCurrentRow(null)
    form.reset({
      principal_code: '',
      principal_name: '',
      business_domain_id: null,
      usage_team_id: null,
    })
    setOpen('create')
  }
  const openUpdate = (row: GovernancePrincipal) => {
    setCurrentRow(row)
    form.reset({
      principal_code: row.principal_code,
      principal_name: row.principal_name,
      business_domain_id: row.business_domain_id,
      usage_team_id: row.usage_team_id,
    })
    setOpen('update')
  }

  const onSubmit = async (data: FormValues) => {
    if (!data.business_domain_id || !data.usage_team_id) return
    setIsSubmitting(true)
    try {
      if (isUpdate && currentRow) {
        await updatePrincipal(currentRow.id, {
          principal_name: data.principal_name,
          business_domain_id: data.business_domain_id,
          usage_team_id: data.usage_team_id,
        })
        toast.success(t('Principal updated'))
      } else {
        await createPrincipal({
          principal_code: data.principal_code,
          principal_name: data.principal_name,
          business_domain_id: data.business_domain_id,
          usage_team_id: data.usage_team_id,
        })
        toast.success(t('Principal created'))
      }
      setOpen(null)
      triggerRefresh()
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
        <p className='max-w-2xl text-sm text-muted-foreground'>
          {t('Principals identify the person accountable for weak-identity keys. Each principal belongs to a business domain and a usage team.')}
        </p>
        <Button size='sm' onClick={openCreate}>
          <Plus className='size-4' />
          {t('Add Principal')}
        </Button>
      </div>

      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No principals found')}
        emptyDescription={t('No principals match the current search and filters.')}
        skeletonKeyPrefix='principals-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by code or name...'),
          searchDebounceMs: 500,
          filters: [
            {
              columnId: 'business_domain_id',
              title: t('Business Domain'),
              options: domainRef.data?.options ?? [],
              singleSelect: true,
            },
            {
              columnId: 'usage_team_id',
              title: t('Usage Team'),
              options: teamRef.data?.options ?? [],
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

      <Sheet
        open={isOpen}
        onOpenChange={(v) => {
          if (!v) {
            setOpen(null)
            form.reset()
          }
        }}
      >
        <SheetContent className={sideDrawerContentClassName('sm:max-w-[520px]')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>
              {isUpdate ? t('Update Principal') : t('Create Principal')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update the principal name, business domain or usage team.')
                : t('Add a principal to identify the person accountable for weak-identity keys.')}{' '}
              {t('Click save when you&apos;re done.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='principal-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName()}
            >
              <fieldset disabled={isSubmitting} className='contents'>
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='principal_code'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Principal Code')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={isUpdate}
                            placeholder={t('e.g. alice, bot-svc')}
                          />
                        </FormControl>
                        <FormDescription>{t(PRINCIPAL_CODE_HELP)}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='principal_name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Principal Name')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('Enter a principal name')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormItem>
                    <FormLabel>{t('Type')}</FormLabel>
                    <FormControl>
                      <Input value={t('Person')} disabled readOnly />
                    </FormControl>
                    <FormDescription>
                      {t('Principal type is fixed to Person for accountable individuals.')}
                    </FormDescription>
                  </FormItem>
                  <FormField
                    control={form.control}
                    name='business_domain_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Business Domain')}</FormLabel>
                        <FormControl>
                          <BusinessDomainSelect
                            value={field.value}
                            onChange={field.onChange}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='usage_team_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Usage Team')}</FormLabel>
                        <FormControl>
                          <UsageTeamSelect
                            value={field.value}
                            onChange={field.onChange}
                          />
                        </FormControl>
                        <FormDescription>
                          {t('The usage team this principal belongs to.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              </fieldset>
            </form>
          </Form>
          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose render={<Button variant='outline' />}>
              {t('Close')}
            </SheetClose>
            <Button form='principal-form' type='submit' disabled={isSubmitting}>
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
