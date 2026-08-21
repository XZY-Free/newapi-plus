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
  createApplication,
  listApplications,
  listBusinessDomains,
  listOwnerTeams,
  updateApplication,
} from '../../api'
import {
  getDomainCodeSchema,
  getNameSchema,
} from '../../lib/code-schema'
import { parseEnabledFilter } from '../../lib/enabled-filter'
import { useGovernanceTableState } from '../../lib/governance-table-state'
import { useMasterDataReference } from '../../lib/master-data-reference'
import type {
  GovernanceApplication,
  UpdateApplicationPayload,
} from '../../types'
import { EnabledBadge } from '../enabled-badge'
import {
  BusinessDomainSelect,
  OwnerTeamSelect,
} from '../master-data-selects'
import { MasterDataRowActions } from '../master-data-row-actions'

const APP_CODE_HELP =
  'The app code is a stable machine identifier that will be used as root_app_id. Starts with a lowercase letter; lowercase letters, numbers, dots, underscores or hyphens; 2-64 characters. Do not change it after creation.'

type FormValues = {
  app_code: string
  app_name: string
  business_domain_id: number | null
  owner_team_id: number | null
}

function requiredId(t: (key: string) => string, msg: string) {
  return z
    .number()
    .nullable()
    .refine((v) => v != null, t(msg))
}

/**
 * AI 应用分区页（§11-B §8）。
 *
 * app_code 是稳定机器标识（将作为 root_app_id），创建后只读、不得随意修改；
 * 创建/编辑用 Business Domain Selector + Owner Team Selector；不得把 Usage Team
 * 当 Owner Team；列含 business_domain / owner_team（引用数据解析名称）；筛选
 * keyword/enabled/business_domain_id/owner_team_id。
 */
export function ApplicationsPage() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const [open, setOpen] = useDialogState<'create' | 'update' | null>(null)
  const [currentRow, setCurrentRow] = useState<GovernanceApplication | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  const tableState = useGovernanceTableState(isMobile ? 10 : 20)

  const domainRef = useMasterDataReference({
    queryKey: ['ai-governance', 'business-domains'],
    fetchPage: listBusinessDomains,
    itemToValue: (item) => item.id,
    itemToLabel: (item) => item.domain_name,
  })
  const ownerTeamRef = useMasterDataReference({
    queryKey: ['ai-governance', 'owner-teams'],
    fetchPage: listOwnerTeams,
    itemToValue: (item) => item.id,
    itemToLabel: (item) => item.team_name,
  })

  const columns: ColumnDef<GovernanceApplication>[] = [
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
      accessorKey: 'app_code',
      header: t('App Code'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <code className='text-sm font-medium'>{row.original.app_code}</code>
      ),
      size: 180,
    },
    {
      accessorKey: 'app_name',
      header: t('App Name'),
      cell: ({ row }) => (
        <span className='font-medium'>{row.original.app_name}</span>
      ),
      size: 200,
    },
    {
      accessorKey: 'business_domain_id',
      header: t('Business Domain'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {domainRef.isError ? (
            '—'
          ) : (
            (domainRef.data?.byId.get(row.original.business_domain_id) ??
              String(row.original.business_domain_id))
          )}
        </span>
      ),
      filterFn: (row, _id, value: unknown) =>
        String(row.original.business_domain_id) === value,
      size: 160,
    },
    {
      accessorKey: 'owner_team_id',
      header: t('Owner Team'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {ownerTeamRef.isError ? (
            '—'
          ) : (
            (ownerTeamRef.data?.byId.get(row.original.owner_team_id) ??
              String(row.original.owner_team_id))
          )}
        </span>
      ),
      filterFn: (row, _id, value: unknown) =>
        String(row.original.owner_team_id) === value,
      size: 160,
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
                await updateApplication(entity.id, { enabled })
                toast.success(
                  t(enabled ? 'Application enabled' : 'Application disabled')
                )
                triggerRefresh()
              } catch (error) {
                handleServerError(error)
                throw error
              }
            }}
            enableDesc={t('This AI application will be available for assignment.')}
            disableDesc={t('This AI application will no longer be selectable. Existing references are preserved.')}
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
  const ownerTeamFilter = tableState.columnFilters.find(
    (f) => f.id === 'owner_team_id'
  )?.value as string | undefined
  const enabledFilter = tableState.columnFilters.find(
    (f) => f.id === 'enabled'
  )?.value as string | undefined
  const enabledValue = parseEnabledFilter(enabledFilter)

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: [
      'ai-governance',
      'applications',
      tableState.pagination.pageIndex + 1,
      tableState.pagination.pageSize,
      tableState.globalFilter,
      domainFilter,
      ownerTeamFilter,
      enabledValue,
      refreshTrigger,
    ],
    queryFn: () =>
      listApplications({
        page: tableState.pagination.pageIndex + 1,
        page_size: tableState.pagination.pageSize,
        keyword: tableState.globalFilter || undefined,
        business_domain_id: domainFilter ? Number(domainFilter) : undefined,
        owner_team_id: ownerTeamFilter ? Number(ownerTeamFilter) : undefined,
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
    app_code: getDomainCodeSchema(t),
    app_name: getNameSchema(t),
    business_domain_id: requiredId(t, 'Please select a business domain'),
    owner_team_id: requiredId(t, 'Please select an owner team'),
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      app_code: '',
      app_name: '',
      business_domain_id: null,
      owner_team_id: null,
    },
  })

  const isUpdate = open === 'update'
  const isOpen = open !== null
  const [isSubmitting, setIsSubmitting] = useState(false)

  const openCreate = () => {
    setCurrentRow(null)
    form.reset({
      app_code: '',
      app_name: '',
      business_domain_id: null,
      owner_team_id: null,
    })
    setOpen('create')
  }
  const openUpdate = (row: GovernanceApplication) => {
    setCurrentRow(row)
    form.reset({
      app_code: row.app_code,
      app_name: row.app_name,
      business_domain_id: row.business_domain_id,
      owner_team_id: row.owner_team_id,
    })
    setOpen('update')
  }

  const onSubmit = async (data: FormValues) => {
    if (!data.business_domain_id || !data.owner_team_id) return
    setIsSubmitting(true)
    try {
      if (isUpdate && currentRow) {
        // 只发送实际修改的字段，绝不重传未变的关联 ID（见 principals 页同款说明）。
        const payload: UpdateApplicationPayload = {}
        if (data.app_name !== currentRow.app_name) {
          payload.app_name = data.app_name
        }
        if (data.business_domain_id !== currentRow.business_domain_id) {
          payload.business_domain_id = data.business_domain_id
        }
        if (data.owner_team_id !== currentRow.owner_team_id) {
          payload.owner_team_id = data.owner_team_id
        }
        await updateApplication(currentRow.id, payload)
        toast.success(t('Application updated'))
      } else {
        await createApplication({
          app_code: data.app_code,
          app_name: data.app_name,
          business_domain_id: data.business_domain_id,
          owner_team_id: data.owner_team_id,
        })
        toast.success(t('Application created'))
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
          {t('AI applications carry a stable app code used as root_app_id. Each application belongs to a business domain and is owned by an application owner team.')}
        </p>
        <Button size='sm' onClick={openCreate}>
          <Plus className='size-4' />
          {t('Add AI Application')}
        </Button>
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
        emptyTitle={t('No AI applications found')}
        emptyDescription={t('No AI applications match the current search and filters.')}
        skeletonKeyPrefix='applications-skeleton'
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
              columnId: 'owner_team_id',
              title: t('Owner Team'),
              options: ownerTeamRef.data?.options ?? [],
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
              {isUpdate ? t('Update AI Application') : t('Create AI Application')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update the application name, business domain or owner team.')
                : t('Add an AI application with a stable app code.')}{' '}
              {t('Click save when you&apos;re done.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='application-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName()}
            >
              <fieldset disabled={isSubmitting} className='contents'>
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='app_code'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('App Code')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={isUpdate}
                            placeholder={t('e.g. hr-chat, finance-ai')}
                          />
                        </FormControl>
                        <FormDescription>{t(APP_CODE_HELP)}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='app_name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('App Name')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('Enter an application name')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
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
                            defaultLabel={
                              isUpdate && currentRow
                                ? domainRef.data?.byId.get(
                                    currentRow.business_domain_id
                                  )
                                : undefined
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='owner_team_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Owner Team')}</FormLabel>
                        <FormControl>
                          <OwnerTeamSelect
                            value={field.value}
                            onChange={field.onChange}
                            defaultLabel={
                              isUpdate && currentRow
                                ? ownerTeamRef.data?.byId.get(
                                    currentRow.owner_team_id
                                  )
                                : undefined
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('The application owner team responsible for building, maintaining and operating this application.')}
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
            <Button form='application-form' type='submit' disabled={isSubmitting}>
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
