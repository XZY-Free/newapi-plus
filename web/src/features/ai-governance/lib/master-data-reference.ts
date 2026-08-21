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

import type { MasterDataFetchParams } from './master-data-loader'
import type { PagedResult } from '../types'

/**
 * 后端单页分页上限（各主数据 list 接口一致）。
 */
export const MASTER_DATA_REFERENCE_PAGE_SIZE = 200

/**
 * 主数据「引用」加载（§11-B §9，表格列名解析 + 列表筛选下拉）。
 *
 * Principal / Application 列表行仅含 `*_id`，不含名称。此 hook 一次性拉取引用实体，
 * 返回 `byId`（id→label）供表格列解析，与 `options`（label/value）供列表筛选下拉。
 *
 * 注意：这是「引用数据」（用于列展示与筛选选项），必须按后端 `total` 分页拉全量，
 * 不得用单页大 `page_size` 假取全（后端每页上限 {@link MASTER_DATA_REFERENCE_PAGE_SIZE}）；
 * 空页/异常 `total` 安全终止。与表单中的 `MasterDataSelect`（分页搜索加载）用途不同，
 * 引用不按 enabled 过滤，历史已停用的关联仍能解析名称。
 */
export function useMasterDataReference<T>({
  queryKey,
  fetchPage,
  itemToValue,
  itemToLabel,
  pageSize = MASTER_DATA_REFERENCE_PAGE_SIZE,
}: {
  queryKey: string[]
  fetchPage: (params: MasterDataFetchParams) => Promise<PagedResult<T>>
  itemToValue: (item: T) => number
  itemToLabel: (item: T) => string
  pageSize?: number
}) {
  return useQuery({
    queryKey: [...queryKey, 'reference'],
    queryFn: async () => {
      const first = await fetchPage({ page: 1, page_size: pageSize })
      const items = [...first.items]
      const total = first.total
      // 第一页已覆盖全部（含异常 total：如 items.length >= total 或 total 为 0 却有条目）。
      if (items.length >= total) return items
      const pageCount = Math.ceil(total / pageSize)
      if (pageCount <= 1) return items
      const rest = await Promise.all(
        Array.from({ length: pageCount - 1 }, (_, i) =>
          fetchPage({ page: i + 2, page_size: pageSize })
        )
      )
      return [...items, ...rest.flatMap((page) => page.items)]
    },
    select: (data) => ({
      byId: new Map<number, string>(
        data.map((item) => [itemToValue(item), itemToLabel(item)])
      ),
      options: data.map((item) => ({
        label: itemToLabel(item),
        value: String(itemToValue(item)),
      })),
    }),
  })
}
