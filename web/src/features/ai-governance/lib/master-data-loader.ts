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
import { useInfiniteQuery } from '@tanstack/react-query'

import type { PagedResult } from '../types'

/**
 * 主数据选择器分页请求参数。后端 `page/page_size/keyword` 均支持，见
 * `GovernanceListQuery`。选择器绝不假设主数据永远少于第一页数量。
 */
export interface MasterDataFetchParams {
  page: number
  page_size: number
  keyword?: string
}

export interface UseMasterDataOptionsArgs<T> {
  queryKey: string[]
  fetchPage: (params: MasterDataFetchParams) => Promise<PagedResult<T>>
  keyword?: string
  pageSize?: number
}

/**
 * 受控多页加载主数据选项（§11-B §八）。
 *
 * 基于 `useInfiniteQuery`：打开即从第 1 页加载；`keyword` 变化会改变 queryKey，
 * 自动重置回第 1 页；`hasNextPage` 依据后端 `total` 判断，配合「加载更多」翻页。
 *
 * 返回 `data` 为已聚合的扁平 items 数组。
 */
export function useMasterDataOptions<T>({
  queryKey,
  fetchPage,
  keyword,
  pageSize = 50,
}: UseMasterDataOptionsArgs<T>) {
  return useInfiniteQuery({
    queryKey: [...queryKey, keyword ?? ''],
    initialPageParam: 1,
    queryFn: ({ pageParam }) =>
      fetchPage({ page: pageParam, page_size: pageSize, keyword }),
    getNextPageParam: (lastPage, allPages) => {
      // React Query 会在首页解析前对乐观结果调用 getNextPageParam（此时 lastPage 为
      // undefined）。防御性短路，避免 in-flight 状态下读取 lastPage.items 抛错。
      if (!lastPage) return undefined
      const loaded = allPages.reduce((sum, page) => sum + page.items.length, 0)
      return loaded < lastPage.total ? allPages.length + 1 : undefined
    },
    select: (data) => data.pages.flatMap((page) => page.items),
  })
}
