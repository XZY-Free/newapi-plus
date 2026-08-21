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
 * 主数据「引用」加载（§11-B §9，表格列名解析 + 列表筛选下拉）。
 *
 * Principal / Application 列表行仅含 `*_id`，不含名称。此 hook 一次性拉取引用实体，
 * 返回 `byId`（id→label）供表格列解析，与 `options`（label/value）供列表筛选下拉。
 *
 * 注意：这是「引用数据」（用于列展示与筛选选项），拉取一大页（默认 500）即可；
 * 与表单中的 `MasterDataSelect`（分页搜索加载，绝不写死第一页）用途不同。
 */
export function useMasterDataReference<T>({
  queryKey,
  fetchPage,
  itemToValue,
  itemToLabel,
  pageSize = 500,
}: {
  queryKey: string[]
  fetchPage: (params: MasterDataFetchParams) => Promise<PagedResult<T>>
  itemToValue: (item: T) => number
  itemToLabel: (item: T) => string
  pageSize?: number
}) {
  return useQuery({
    queryKey: [...queryKey, 'reference'],
    queryFn: () => fetchPage({ page: 1, page_size: pageSize }),
    select: (data) => ({
      byId: new Map<number, string>(
        data.items.map((item) => [itemToValue(item), itemToLabel(item)])
      ),
      options: data.items.map((item) => ({
        label: itemToLabel(item),
        value: String(itemToValue(item)),
      })),
    }),
  })
}
