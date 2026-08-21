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

import { getNameSchema } from '../lib/code-schema'
import { parseEnabledFilter } from '../lib/enabled-filter'
import { useGovernanceTableState } from '../lib/governance-table-state'
import { EnabledBadge } from './enabled-badge'
import { MasterDataRowActions } from './master-data-row-actions'

/** 泛型行基线：稳定主数据行均具备 id 与 enabled。 */
export type MasterDataCodeCrudRow = {
  id: number
  enabled: boolean
}

type FormValues = {
  code: string
  name: string
}

/**
 * 三个“code + name + enabled”主数据分区的统一 CRUD 页面（§11-B §3 §4 §7）。
 *
 * Business Domains / Usage Teams / Owner Teams 结构完全一致，仅实体、API 与文案不同，
 * 故收敛为一份具体可复用组件（非元数据表单引擎）：入参为配置对象，组件内部统一
 * 承担 provider 状态、列、表格、创建/编辑抽屉与启停确认。
 *
 * - code 创建后只读：编辑态 `disabled` 且不随提交。
 * - enabled 统一「启用/停用」语义，带确认；后端拒绝停用时错误透出真实 message。
 * - 消费真实 `PagedResult`，分页/搜索/筛选回第 1 页由 `useGovernanceTableState` 承担。
 * - 无删除能力。
 */
export function MasterDataCodeCrudPage<
  T extends MasterDataCodeCrudRow,
  TCreate,
  TUpdate,
>({
  queryKey,
  list,
  create,
  update,
  getCode,
  getName,
  getUpdatedAt,
  toCreate,
  toUpdate,
  toToggle,
  toDefaults,
  /** 前端预校验 code 用的 zod schema（domain/app 用严格、team/principal 用简单）。 */
  codeSchema,
  // i18n（入参为英文源串，组件内 t() 取 7 语言翻译）
  codeLabel,
  nameLabel,
  codePlaceholder,
  namePlaceholder,
  codeHelp,
  pageDescription,
  emptyTitle,
  emptyDescription,
  searchPlaceholder,
  addButton,
  createTitle,
  updateTitle,
  createDescription,
  updateDescription,
  entityCreated,
  entityUpdated,
  entityEnabled,
  entityDisabled,
  enableDesc,
  disableDesc,
}: {
  queryKey: string[]
  list: (q: {
    page?: number
    page_size?: number
    keyword?: string
    enabled?: boolean
  }) => Promise<{ items: T[]; total: number }>
  create: (payload: TCreate) => Promise<T>
  update: (id: number, payload: TUpdate) => Promise<T>
  getCode: (row: T) => string
  getName: (row: T) => string
  getUpdatedAt: (row: T) => number
  toCreate: (form: FormValues) => TCreate
  toUpdate: (form: FormValues) => TUpdate
  toToggle: (row: T, enabled: boolean) => TUpdate
  toDefaults: (row: T) => FormValues
  /** 前端预校验 code 用的 zod schema（domain/app 用严格、team/principal 用简单）。 */
  codeSchema: z.ZodString
  codeLabel: string
  nameLabel: string
  codePlaceholder: string
  namePlaceholder: string
  codeHelp: string
  pageDescription: string
  emptyTitle: string
  emptyDescription: string
  searchPlaceholder: string
  addButton: string
  createTitle: string
  updateTitle: string
  createDescription: string
  updateDescription: string
  entityCreated: string
  entityUpdated: string
  entityEnabled: string
  entityDisabled: string
  enableDesc: string
  disableDesc: string
}) {
  const { t } = useTranslation()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const [open, setOpen] = useDialogState<'create' | 'update' | null>(null)
  const [drawerEntity, setDrawerEntity] = useState<T | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  const tableState = useGovernanceTableState(isMobile ? 10 : 20)

  const handleToggle = async (row: T, enabled: boolean) => {
    try {
      await update(row.id, toToggle(row, enabled))
      toast.success(t(enabled ? entityEnabled : entityDisabled))
      triggerRefresh()
    } catch (error) {
      handleServerError(error)
      throw error
    }
  }

  const columns: ColumnDef<T>[] = [
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
      id: 'code',
      header: t(codeLabel),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <code className='text-sm font-medium'>{getCode(row.original)}</code>
      ),
      size: 200,
    },
    {
      id: 'name',
      header: t(nameLabel),
      cell: ({ row }) => (
        <span className='font-medium'>{getName(row.original)}</span>
      ),
      size: 240,
    },
    {
      id: 'enabled',
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
      id: 'updated_at',
      header: t('Updated'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[160px] font-mono text-sm'>
          {formatTimestamp(getUpdatedAt(row.original))}
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
            onToggle={(enabled) => handleToggle(entity, enabled)}
            enableDesc={t(enableDesc)}
            disableDesc={t(disableDesc)}
          />
        )
      },
      meta: { pinned: 'right' as const },
      size: 120,
    },
  ]

  const enabledFilter = tableState.columnFilters.find(
    (f) => f.id === 'enabled'
  )?.value as string | undefined
  const enabledValue = parseEnabledFilter(enabledFilter)

  // 列表加载
  const { data, isLoading, isFetching, error, refetch } = useQueryData<T>({
    queryKey,
    page: tableState.pagination.pageIndex + 1,
    pageSize: tableState.pagination.pageSize,
    keyword: tableState.globalFilter || undefined,
    enabled: enabledValue,
    refreshTrigger,
    list,
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

  const formSchema = z.object({
    code: codeSchema,
    name: getNameSchema(t),
  })
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { code: '', name: '' },
  })

  const isUpdate = open === 'update'
  const isOpen = open !== null

  // 打开时填充
  const openCreate = () => {
    setDrawerEntity(null)
    form.reset({ code: '', name: '' })
    setOpen('create')
  }
  const openUpdate = (row: T) => {
    setDrawerEntity(row)
    form.reset(toDefaults(row))
    setOpen('update')
  }

  const [isSubmitting, setIsSubmitting] = useState(false)
  const onSubmit = async (data: FormValues) => {
    setIsSubmitting(true)
    try {
      if (isUpdate && drawerEntity) {
        await update(drawerEntity.id, toUpdate(data))
        toast.success(t(entityUpdated))
      } else {
        await create(toCreate(data))
        toast.success(t(entityCreated))
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
          {t(pageDescription)}
        </p>
        <Button size='sm' onClick={openCreate}>
          <Plus className='size-4' />
          {t(addButton)}
        </Button>
      </div>

      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        isError={error != null && data == null}
        errorTitle={t('Oops! Something went wrong')}
        errorDescription={t('Failed to load')}
        onErrorRetry={() => void refetch()}
        emptyTitle={t(emptyTitle)}
        emptyDescription={t(emptyDescription)}
        skeletonKeyPrefix={`${queryKey.at(-1)}-skeleton`}
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t(searchPlaceholder),
          searchDebounceMs: 500,
          filters: [
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
              {isUpdate ? t(updateTitle) : t(createTitle)}
            </SheetTitle>
            <SheetDescription>
              {isUpdate ? t(updateDescription) : t(createDescription)}{' '}
              {t('Click save when you&apos;re done.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='master-data-code-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName()}
            >
              <fieldset disabled={isSubmitting} className='contents'>
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='code'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t(codeLabel)}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={isUpdate}
                            placeholder={t(codePlaceholder)}
                          />
                        </FormControl>
                        <FormDescription>{t(codeHelp)}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t(nameLabel)}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t(namePlaceholder)} />
                        </FormControl>
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
              form='master-data-code-form'
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

/**
 * 列表加载（分离出来避免泛型组件内联 useQuery 时的类型噪音）。
 * queryKey 变化（含 refreshTrigger）自动重新拉取；placeholderData 保留上一页。
 */
function useQueryData<T>({
  queryKey,
  page,
  pageSize,
  keyword,
  enabled,
  refreshTrigger,
  list,
}: {
  queryKey: string[]
  page: number
  pageSize: number
  keyword?: string
  enabled?: boolean
  refreshTrigger: number
  list: (q: {
    page?: number
    page_size?: number
    keyword?: string
    enabled?: boolean
  }) => Promise<{ items: T[]; total: number }>
}) {
  return useQuery({
    queryKey: [...queryKey, page, pageSize, keyword, enabled, refreshTrigger],
    queryFn: () =>
      list({ page, page_size: pageSize, keyword, enabled }),
    placeholderData: (prev) => prev,
  })
}
