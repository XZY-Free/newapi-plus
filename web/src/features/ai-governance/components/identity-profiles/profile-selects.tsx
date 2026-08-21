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
import { Check } from 'lucide-react'
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

import { listApplications, listCredentialPurposes, listPrincipals } from '../../api'
import {
  useMasterDataOptions,
  type MasterDataFetchParams,
} from '../../lib/master-data-loader'
import type {
  GovernanceApplication,
  GovernanceCredentialPurpose,
  GovernancePrincipal,
} from '../../types'
import { MasterDataSelect } from '../master-data-select'

// ---------------------------------------------------------------------------
// 单选：Principal / Credential Purpose（新候选仅 enabled=true）
// ---------------------------------------------------------------------------

type SingleSelectProps = {
  id?: string
  value: number | null
  onChange: (value: number | null) => void
  /** 当前已关联（历史可能已停用）的名称，编辑态预填。 */
  defaultLabel?: string
  disabled?: boolean
  className?: string
}

/**
 * 使用主体选择器（§11-C §六）。仅 STATIC/PRINCIPAL 显示并要求；
 * 新候选仅 enabled=true；历史已停用的当前值仍以 defaultLabel 可识别显示。
 */
export function PrincipalSelect(props: SingleSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<GovernancePrincipal>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'principal-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listPrincipals({ page, page_size, keyword, enabled: true })
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.principal_name}
      placeholder={t('Select principal')}
      emptyText={t('No principal found')}
    />
  )
}

/**
 * 凭证用途选择器（§11-C §七）。仅 STATIC/PRINCIPAL 显示并要求；
 * 保持企业语义：用途 = 这把 API Key 被批准用于什么，绝不代表客户端已通过技术验证。
 */
export function PurposeSelect(props: SingleSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<GovernanceCredentialPurpose>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'credential-purpose-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listCredentialPurposes({ page, page_size, keyword, enabled: true })
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.purpose_name}
      placeholder={t('Select credential purpose')}
      emptyText={t('No credential purpose found')}
    />
  )
}

// ---------------------------------------------------------------------------
// 多选：AI Application（异步搜索 + 分页加载，新候选仅 enabled=true）
// ---------------------------------------------------------------------------

type ApplicationMultiSelectProps = {
  id?: string
  value: number[]
  onChange: (value: number[]) => void
  disabled?: boolean
  className?: string
}

/**
 * AI 应用多选（§11-C §八，轻量实现，不抽象成新通用表单框架）。
 *
 * 基于 Command + Popover + `useMasterDataOptions`（与 `MasterDataSelect` 同源），
 * 异步搜索 + 分页加载，只展示 enabled=true 的应用；绝不一次拉前 200 条假装全部。
 * 点击候选在 `value`（number[]）中增删。空态四态区分：候选请求失败必须显示
 * Error + Retry，绝不冒充 Empty。
 */
export function ApplicationMultiSelect(props: ApplicationMultiSelectProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')

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
  } = useMasterDataOptions<GovernanceApplication>({
    queryKey: ['ai-governance', 'application-multi-options'],
    fetchPage: ({ page, page_size, keyword }: MasterDataFetchParams) =>
      listApplications({ page, page_size, keyword, enabled: true }),
    keyword: debouncedQuery,
  })

  const selected = new Set(props.value)
  const toggle = (id: number) => {
    const next = new Set(props.value)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    props.onChange([...next])
  }

  const selectedItems = items?.filter((item) => selected.has(item.id)) ?? []
  const selectedCount = props.value.length
  let triggerLabel: string
  if (selectedCount === 0) {
    triggerLabel = t('Select applications')
  } else if (selectedCount === 1) {
    triggerLabel =
      selectedItems[0]?.app_name ?? `${t('Application')} #${props.value[0]}`
  } else {
    triggerLabel = t('{{n}} applications selected', { n: selectedCount })
  }

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
    emptyContent = t('Loading...')
  } else {
    emptyContent = t('No application found')
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
            aria-label={triggerLabel}
            disabled={props.disabled}
            className={cn(
              'border-input text-muted-foreground hover:bg-muted/55 hover:text-foreground w-full justify-between rounded-lg px-3 py-2 font-normal',
              open && 'border-ring ring-ring/20 text-foreground ring-[3px]',
              selectedCount > 0 && 'text-foreground',
              props.className
            )}
          />
        }
      >
        <span className='truncate'>{triggerLabel}</span>
      </PopoverTrigger>
      <PopoverContent
        className='data-closed:zoom-out-100 data-open:zoom-in-100 data-[side=bottom]:slide-in-from-top-0 data-[side=left]:slide-in-from-right-0 data-[side=right]:slide-in-from-left-0 data-[side=top]:slide-in-from-bottom-0 w-[var(--anchor-width)] overflow-hidden rounded-xl p-0 shadow-lg data-closed:duration-75 data-open:duration-100'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            id={props.id}
            placeholder={t('Search...')}
            value={query}
            onValueChange={setQuery}
          />
          <CommandList className='max-h-[300px]'>
            <CommandEmpty>{emptyContent}</CommandEmpty>
            {items?.map((item) => (
              <CommandItem
                key={item.id}
                value={String(item.id)}
                onSelect={() => toggle(item.id)}
                className='gap-2'
              >
                <Check
                  aria-hidden='true'
                  className={cn(
                    'size-4',
                    selected.has(item.id) ? 'opacity-100' : 'opacity-0'
                  )}
                />
                <span className='truncate'>{item.app_name}</span>
                <span className='text-muted-foreground truncate text-xs'>
                  {item.app_code}
                </span>
              </CommandItem>
            ))}
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
                    ? t('Loading...')
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
