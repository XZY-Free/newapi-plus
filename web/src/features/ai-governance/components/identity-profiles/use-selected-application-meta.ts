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
import { useQueries } from '@tanstack/react-query'

import { getApplication } from '../../api'
import type { GovernanceBindingView } from '../../types'

/**
 * 已选应用当前事实（§11-C §C.5 P1-A）。
 *
 * `bindings[].enabled` 是 Binding 行自身的 enabled，**不是** AI Application 当前的
 * enabled。本 Hook 按 `bindings[].app_id`（去重后）逐一 `getApplication(id)` 精确读取
 * Application 当前事实，绝不用「不在 enabled 候选页」来推断 Application disabled。
 *
 * 返回 `enabled: boolean | null`：
 * - `true`  → 显示 Enabled
 * - `false` → 显示 Disabled
 * - `null`  → 加载中 / 读取失败（Unknown/Error），**不得冒充 Disabled**
 *
 * 读取失败时 `app_name`/`app_code` 保留 binding 快照作显示，但 enabled 绝不猜测。
 *
 * ApplicationMultiSelect 的**新候选**仍走 `listApplications(enabled=true)` + 服务端搜索
 * + 分页，本 Hook 不改变候选行为，只修正「已绑定应用」的当前事实解析。
 */
export type SelectedApplicationFact = {
  app_name: string
  app_code: string
  /** Application 当前 enabled；null = 加载中 / 读取失败（Unknown）。 */
  enabled: boolean | null
}

export function useSelectedApplicationMeta(
  bindings: Pick<
    GovernanceBindingView,
    'app_id' | 'app_name' | 'app_code'
  >[]
): Record<number, SelectedApplicationFact> {
  const appIds = [...new Set(bindings.map((b) => b.app_id))]
  const results = useQueries({
    queries: appIds.map((appId) => ({
      queryKey: ['ai-governance', 'application', appId],
      queryFn: () => getApplication(appId),
      staleTime: 60_000,
    })),
  })

  const byId = new Map<number, SelectedApplicationFact>()
  appIds.forEach((appId, i) => {
    const r = results[i]
    if (r?.data) {
      byId.set(appId, {
        app_name: r.data.app_name,
        app_code: r.data.app_code,
        enabled: r.data.enabled,
      })
    } else {
      // 加载中 / 读取失败：name/app_code 用 binding 快照，enabled 为 null（未知），
      // 绝不冒充 Disabled。
      const b = bindings.find((x) => x.app_id === appId)
      byId.set(appId, {
        app_name: b?.app_name ?? '',
        app_code: b?.app_code ?? '',
        enabled: null,
      })
    }
  })

  // 所有 binding 的 app_id 都来自 appIds，byId 必有键；`??` 仅作防御性兜底。
  const meta: Record<number, SelectedApplicationFact> = {}
  for (const b of bindings) {
    meta[b.app_id] = byId.get(b.app_id) ?? {
      app_name: b.app_name,
      app_code: b.app_code,
      enabled: null,
    }
  }
  return meta
}
