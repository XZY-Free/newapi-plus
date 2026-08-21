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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  createCredentialPurpose,
  listCredentialPurposes,
  updateCredentialPurpose,
} from '../../api'
import {
  getNameSchema,
  getSimpleCodeSchema,
  SIMPLE_CODE_MAX_LENGTH,
} from '../../lib/code-schema'
import {
  getPurposeTypeLabel,
  purposeTypeOptions,
  purposeTypeSchema,
} from '../../lib/credential-purpose-types'
import { parseEnabledFilter } from '../../lib/enabled-filter'
import { useGovernanceTableState } from '../../lib/governance-table-state'
import type {
  CredentialPurposeType,
  GovernanceCredentialPurpose,
} from '../../types'
import { EnabledBadge } from '../enabled-badge'
import { MasterDataRowActions } from '../master-data-row-actions'

const PURPOSE_CODE_HELP =
  'Required, no whitespace, up to 64 characters. Cannot be changed after creation.'
const PURPOSE_DESC =
  'A credential purpose declares what a key is approved for (Desktop Client, IDE, Script, Service, Other). This is an approved use, not a verified client.'

type FormValues = {
  purpose_code: string
  purpose_name: string
  purpose_type: CredentialPurposeType | ''
}

/**
 * 凭证用途分区页（§11-B §6）。
 *
 * purpose_type 枚举：DESKTOP_CLIENT→桌面客户端 / IDE / SCRIPT→脚本 / SERVICE→服务 /
 * OTHER→其他；提交与筛选保枚举原值。帮助文本明确「这是批准用途，不是已验证的客户端」，
 * 不出现「Verified WorkBuddy」等误导文案。
 */
export function CredentialPurposesPage() {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const [open, setOpen] = useDialogState<'create' | 'update' | null>(null)
  const [currentRow, setCurrentRow] = useState<GovernanceCredentialPurpose | null>(
    null
  )
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  const tableState = useGovernanceTableState(isMobile ? 10 : 20)

  const columns: ColumnDef<GovernanceCredentialPurpose>[] = [
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
      accessorKey: 'purpose_code',
      header: t('Purpose Code'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <code className='text-sm font-medium'>
          {row.original.purpose_code}
        </code>
      ),
      size: 200,
    },
    {
      accessorKey: 'purpose_name',
      header: t('Purpose Name'),
      cell: ({ row }) => (
        <span className='font-medium'>{row.original.purpose_name}</span>
      ),
      size: 220,
    },
    {
      accessorKey: 'purpose_type',
      header: t('Purpose Type'),
      cell: ({ row }) => (
        <span className='text-sm'>{getPurposeTypeLabel(t, row.original.purpose_type)}</span>
      ),
      filterFn: (row, _id, value: unknown) =>
        row.original.purpose_type === value,
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
                await updateCredentialPurpose(entity.id, { enabled })
                toast.success(
                  t(enabled ? 'Credential purpose enabled' : 'Credential purpose disabled')
                )
                triggerRefresh()
              } catch (error) {
                handleServerError(error)
                throw error
              }
            }}
            enableDesc={t('This credential purpose will be available for assignment.')}
            disableDesc={t('This credential purpose will no longer be selectable. Existing references are preserved.')}
          />
        )
      },
      meta: { pinned: 'right' as const },
      size: 120,
    },
  ]

  const purposeTypeFilter = tableState.columnFilters.find(
    (f) => f.id === 'purpose_type'
  )?.value as string | undefined
  const enabledFilter = tableState.columnFilters.find(
    (f) => f.id === 'enabled'
  )?.value as string | undefined
  const enabledValue = parseEnabledFilter(enabledFilter)

  const { data, isLoading, isFetching, isError, refetch } = useQuery({
    queryKey: [
      'ai-governance',
      'credential-purposes',
      tableState.pagination.pageIndex + 1,
      tableState.pagination.pageSize,
      tableState.globalFilter,
      purposeTypeFilter,
      enabledValue,
      refreshTrigger,
    ],
    queryFn: () =>
      listCredentialPurposes({
        page: tableState.pagination.pageIndex + 1,
        page_size: tableState.pagination.pageSize,
        keyword: tableState.globalFilter || undefined,
        purpose_type: purposeTypeFilter as CredentialPurposeType | undefined,
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
    purpose_code: getSimpleCodeSchema(t, SIMPLE_CODE_MAX_LENGTH),
    purpose_name: getNameSchema(t),
    purpose_type: purposeTypeSchema
      .or(z.literal(''))
      .refine((v) => v !== '', t('Please select a purpose type')),
  })

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { purpose_code: '', purpose_name: '', purpose_type: '' },
  })

  const isUpdate = open === 'update'
  const isOpen = open !== null
  const [isSubmitting, setIsSubmitting] = useState(false)

  const openCreate = () => {
    setCurrentRow(null)
    form.reset({ purpose_code: '', purpose_name: '', purpose_type: '' })
    setOpen('create')
  }
  const openUpdate = (row: GovernanceCredentialPurpose) => {
    setCurrentRow(row)
    form.reset({
      purpose_code: row.purpose_code,
      purpose_name: row.purpose_name,
      purpose_type: row.purpose_type,
    })
    setOpen('update')
  }

  const onSubmit = async (data: FormValues) => {
    if (!data.purpose_type) return
    setIsSubmitting(true)
    try {
      if (isUpdate && currentRow) {
        await updateCredentialPurpose(currentRow.id, {
          purpose_name: data.purpose_name,
          purpose_type: data.purpose_type,
        })
        toast.success(t('Credential purpose updated'))
      } else {
        await createCredentialPurpose({
          purpose_code: data.purpose_code,
          purpose_name: data.purpose_name,
          purpose_type: data.purpose_type,
        })
        toast.success(t('Credential purpose created'))
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
        <p className='max-w-2xl text-sm text-muted-foreground'>{t(PURPOSE_DESC)}</p>
        <Button size='sm' onClick={openCreate}>
          <Plus className='size-4' />
          {t('Add Credential Purpose')}
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
        emptyTitle={t('No credential purposes found')}
        emptyDescription={t(
          'No credential purposes match the current search and filters.'
        )}
        skeletonKeyPrefix='credential-purposes-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by code or name...'),
          searchDebounceMs: 500,
          filters: [
            {
              columnId: 'purpose_type',
              title: t('Purpose Type'),
              options: purposeTypeOptions(t),
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
              {isUpdate ? t('Update Credential Purpose') : t('Create Credential Purpose')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update the purpose name, type or status.')
                : t('Add a credential purpose that declares what a key is approved for.')}{' '}
              {t('Click save when you&apos;re done.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='credential-purpose-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName()}
            >
              <fieldset disabled={isSubmitting} className='contents'>
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='purpose_code'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Purpose Code')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={isUpdate}
                            placeholder={t('e.g. desktop-client, ide, script')}
                          />
                        </FormControl>
                        <FormDescription>{t(PURPOSE_CODE_HELP)}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='purpose_name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Purpose Name')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('Enter a purpose name')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='purpose_type'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Purpose Type')}</FormLabel>
                        <FormControl>
                          <Select value={field.value} onValueChange={field.onChange}>
                            <SelectTrigger>
                              <SelectValue placeholder={t('Select a purpose type')} />
                            </SelectTrigger>
                            <SelectContent alignItemWithTrigger={false}>
                              <SelectGroup>
                                {purposeTypeOptions(t).map((option) => (
                                  <SelectItem key={option.value} value={option.value}>
                                    {option.label}
                                  </SelectItem>
                                ))}
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                        </FormControl>
                        <FormDescription>{t(PURPOSE_DESC)}</FormDescription>
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
            <Button
              form='credential-purpose-form'
              type='submit'
              disabled={isSubmitting}
            >
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
