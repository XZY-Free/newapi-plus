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
import type {
  ColumnFiltersState,
  OnChangeFn,
  PaginationState,
  Updater,
} from '@tanstack/react-table'
import { useCallback, useState } from 'react'

function resolveUpdater<T>(updater: Updater<T>, previous: T): T {
  return typeof updater === 'function'
    ? (updater as (old: T) => T)(previous)
    : updater
}

/**
 * 治理主数据表格的本地分页/搜索/列筛选状态（§11-B §11）。
 *
 * 六个主数据分区共用同一个 `/_authenticated/ai-governance/$section` 路由，且该路由
 * 未定义 `validateSearch`——若复用 `useTableUrlState`，六个分区的 URL search 参数
 * 会互相污染。因此这里在分区组件内部维护独立表格状态（每次切换分区组件重挂载，
 * 状态自然重置），满足统一语义：切换筛选/搜索自动回到第 1 页。
 *
 * 返回值与 `useTableUrlState` 对齐，可直接喂给 `useDataTable`。
 */
export function useGovernanceTableState(defaultPageSize = 20) {
  const [globalFilter, setGlobalFilter] = useState('')
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: defaultPageSize,
  })

  const onGlobalFilterChange = useCallback<OnChangeFn<string>>((updater) => {
    setGlobalFilter((prev) => resolveUpdater(updater, prev))
    setPagination((prev) => ({ ...prev, pageIndex: 0 }))
  }, [])

  const onColumnFiltersChange = useCallback<OnChangeFn<ColumnFiltersState>>(
    (updater) => {
      setColumnFilters((prev) => resolveUpdater(updater, prev))
      setPagination((prev) => ({ ...prev, pageIndex: 0 }))
    },
    []
  )

  const onPaginationChange = useCallback<OnChangeFn<PaginationState>>(
    (updater) => {
      setPagination((prev) => resolveUpdater(updater, prev))
    },
    []
  )

  const ensurePageInRange = useCallback((pageCount: number) => {
    setPagination((prev) => {
      if (pageCount > 0 && prev.pageIndex + 1 > pageCount) {
        return { ...prev, pageIndex: Math.max(0, pageCount - 1) }
      }
      return prev
    })
  }, [])

  return {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  }
}
