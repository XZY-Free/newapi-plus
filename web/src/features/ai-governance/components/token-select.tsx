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
import { useTranslation } from 'react-i18next'

import { getApiKeys, searchApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'

import { MasterDataSelect } from './master-data-select'

type TokenSelectProps = {
  id?: string
  value: number | null
  onChange: (value: number | null) => void
  defaultLabel?: string
  disabled?: boolean
  className?: string
}

/**
 * NewAPI Token 选择器（§11-C §C.1）。
 *
 * 复用 §11-B 冻结的 `MasterDataSelect`（异步搜索 + 分页加载），数据源仅
 * `getApiKeys` / `searchApiKeys` **元数据**能力——绝不得调用
 * `fetchTokenKey` / `fetchTokenKeysBatch`（那是拉取明文 Key 的入口）。
 * 本选择器只展示 token 的 `name`，从不渲染 `key`，因此 Identity Governance
 * 页面不会读取到任何 API Key 明文。
 */
export function TokenSelect(props: TokenSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<ApiKey>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'api-key-options']}
      fetchPage={async ({ page, page_size, keyword }) => {
        const q = keyword?.trim()
        const res = q
          ? await searchApiKeys({ keyword: q, p: page, size: page_size })
          : await getApiKeys({ p: page, size: page_size })
        if (!res.data) return { items: [], total: 0, page, page_size }
        return res.data
      }}
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.name}
      placeholder={t('Select API key')}
      emptyText={t('No API Keys Found')}
    />
  )
}
