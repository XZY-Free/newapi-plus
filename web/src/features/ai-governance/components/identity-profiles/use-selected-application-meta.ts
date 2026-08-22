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
import { listApplications } from '../../api'
import {
  useMasterDataOptions,
  type MasterDataFetchParams,
} from '../../lib/master-data-loader'
import type { GovernanceApplication, GovernanceBindingView } from '../../types'

/**
 * 用于「已选应用」元数据（§11-C §C.5 P1-4）。
 *
 * `bindings[].enabled` 是 Binding 行自身的 enabled，**不是** AI Application 当前的
 * enabled。本 Hook 复用现有分页引用能力（`useMasterDataOptions` → `listApplications`）
 * 读取 Application 当前事实：`app_id → application.enabled`。
 *
 * 判定规则：
 * - Application 当前 enabled（在 enabled 候选集中）→ `enabled: true`。
 * - Application 已停用/不在候选集 → `enabled: false`，仍保留真实 name + code，
 *   明确显示 Disabled，不作为新的 enabled candidate，允许移除/替换。
 *
 * 名/码优先取 Application 记录（最新事实），缺失时才回退到 binding 快照。
 */
export function useSelectedApplicationMeta(
  bindings: Pick<
    GovernanceBindingView,
    'app_id' | 'app_name' | 'app_code'
  >[]
): Record<number, { app_name: string; app_code: string; enabled: boolean }> {
  const { data: apps } = useMasterDataOptions<GovernanceApplication>({
    queryKey: ['ai-governance', 'application-reference'],
    fetchPage: ({ page, page_size, keyword }: MasterDataFetchParams) =>
      listApplications({ page, page_size, keyword, enabled: true }),
  })

  const appById = new Map((apps ?? []).map((a) => [a.id, a]))
  const meta: Record<number, { app_name: string; app_code: string; enabled: boolean }> = {}
  for (const b of bindings) {
    const app = appById.get(b.app_id)
    meta[b.app_id] = {
      app_name: app?.app_name ?? b.app_name,
      app_code: app?.app_code ?? b.app_code,
      // Application 当前事实（master data enabled），绝不用 binding.enabled。
      enabled: app?.enabled ?? false,
    }
  }
  return meta
}
