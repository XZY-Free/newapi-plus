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
import { Check, ChevronsUpDown } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'

import {
  useMasterDataOptions,
  type MasterDataFetchParams,
} from '../lib/master-data-loader'
import type { PagedResult } from '../types'

/**
 * 治理主数据选择器（§11-B §八）。
 *
 * 基于 Command + Popover，配合 `useMasterDataOptions` 做**异步搜索 + 分页加载**：
 * 打开即从第 1 页加载，输入关键词触发服务端搜索并重置回第 1 页，
 * 列表底部在有更多页时提供「Load more」翻页。绝不假设主数据少于第一页数量。
 *
 * 对外暴露受控 `value: number | null`（提交给表单的 DB id）与 `onChange`；
 * 显示文本由组件内部缓存，编辑态可用 `defaultLabel` 预填（如已有行的 domain_name）。
 */
export function MasterDataSelect<T>({
  id,
  value,
  onChange,
  queryKey,
  fetchPage,
  itemToValue,
  itemToLabel,
  itemToDescription,
  defaultLabel,
  defaultDescription,
  placeholder,
  emptyText,
  loadingText,
  disabled,
  className,
}: {
  id?: string
  value: number | null
  onChange: (value: number | null) => void
  queryKey: string[]
  fetchPage: (params: MasterDataFetchParams) => Promise<PagedResult<T>>
  itemToValue: (item: T) => number
  itemToLabel: (item: T) => string
  /** 可选次要描述行，渲染在候选标签下方（如 Token ID），用于消歧同名校标。 */
  itemToDescription?: (item: T) => ReactNode
  defaultLabel?: string
  defaultDescription?: string
  placeholder: string
  emptyText: string
  loadingText?: string
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  // 服务端搜索加 400ms debounce：输入不每次按键都打后端，queryKey 仅在防抖后变化。
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [selectedLabel, setSelectedLabel] = useState<string | null>(
    defaultLabel ?? null
  )
  // 选中/编辑回显的稳定身份次要行（§11-C P1）：候选、选中、编辑回显三态都必须
  // 能辨识稳定身份（name + code/id），不能选完就退化成只有 name。
  const [selectedDescription, setSelectedDescription] = useState<ReactNode | null>(
    defaultDescription ?? null
  )

  useEffect(() => {
    const id = setTimeout(() => setDebouncedQuery(query), 400)
    return () => clearTimeout(id)
  }, [query])

  const {
    data: items,
    isFetching,
    isError,
    error,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
    refetch,
  } = useMasterDataOptions<T>({
    queryKey,
    fetchPage,
    keyword: debouncedQuery,
  })

  useEffect(() => {
    if (value === null) {
      setSelectedLabel(null)
      setSelectedDescription(null)
    }
  }, [value])

  useEffect(() => {
    if (defaultLabel != null) {
      setSelectedLabel(defaultLabel)
    }
  }, [defaultLabel])

  useEffect(() => {
    if (defaultDescription != null) {
      setSelectedDescription(defaultDescription)
    }
  }, [defaultDescription])

  const handleSelect = (item: T) => {
    setSelectedLabel(itemToLabel(item))
    setSelectedDescription(itemToDescription ? itemToDescription(item) : null)
    onChange(itemToValue(item))
    setOpen(false)
    setQuery('')
    setDebouncedQuery('')
  }

  const selectedLabelValue =
    selectedLabel ?? (value != null ? String(value) : placeholder)

  // 空态四态区分：候选请求失败必须显示 Error（Failed to load options + Retry），
  // 绝不能冒充 Empty；仅当请求成功且无候选时才显示 emptyText。
  let emptyContent: ReactNode
  if (isError) {
    emptyContent = (
      <div className='flex flex-col items-center gap-1.5 py-2 text-center'>
        <span className='text-sm'>{t('Failed to load options')}</span>
        {error != null && (
          <span className='text-xs text-muted-foreground'>{error.message}</span>
        )}
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => void refetch()}
        >
          {t('Retry')}
        </Button>
      </div>
    )
  } else if (isFetching && !items?.length) {
    emptyContent = loadingText ?? t('Loading...')
  } else {
    emptyContent = emptyText
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            aria-label={selectedLabelValue}
            disabled={disabled}
            className={cn(
              'border-input text-muted-foreground hover:bg-muted/55 hover:text-foreground w-full justify-between rounded-lg px-3 py-2 font-normal',
              open && 'border-ring ring-ring/20 text-foreground ring-[3px]',
              value != null && 'text-foreground',
              className
            )}
          />
        }
      >
        <span className='flex min-w-0 flex-col'>
          <span className='truncate'>{selectedLabelValue}</span>
          {selectedDescription != null && (
            <span className='text-muted-foreground truncate text-xs'>
              {selectedDescription}
            </span>
          )}
        </span>
        <ChevronsUpDown className='size-4 shrink-0 opacity-50' />
      </PopoverTrigger>
      <PopoverContent
        className='data-closed:zoom-out-100 data-open:zoom-in-100 data-[side=bottom]:slide-in-from-top-0 data-[side=left]:slide-in-from-right-0 data-[side=right]:slide-in-from-left-0 data-[side=top]:slide-in-from-bottom-0 w-[var(--anchor-width)] overflow-hidden rounded-xl p-0 shadow-lg data-closed:duration-75 data-open:duration-100'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            id={id}
            placeholder={t('Search...')}
            value={query}
            onValueChange={setQuery}
          />
          <CommandList className='max-h-[300px]'>
            <CommandEmpty>{emptyContent}</CommandEmpty>
            {items?.map((item) => {
              const itemValue = itemToValue(item)
              return (
                <CommandItem
                  key={itemValue}
                  value={String(itemValue)}
                  onSelect={() => handleSelect(item)}
                  className='gap-2'
                >
                  <Check
                    aria-hidden='true'
                    className={cn(
                      'size-4',
                      value === itemValue ? 'opacity-100' : 'opacity-0'
                    )}
                  />
                  <span className='flex min-w-0 flex-col'>
                    <span className='truncate'>{itemToLabel(item)}</span>
                    {itemToDescription && (
                      <span className='text-muted-foreground truncate text-xs'>
                        {itemToDescription(item)}
                      </span>
                    )}
                  </span>
                </CommandItem>
              )
            })}
            {hasNextPage && (
              <div className='border-t p-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='w-full'
                  onClick={() => void fetchNextPage()}
                  disabled={isFetchingNextPage}
                >
                  {isFetchingNextPage
                    ? (loadingText ?? t('Loading...'))
                    : t('Load more')}
                </Button>
              </div>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
