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

import { listIdentityProfiles } from '../api'

/**
 * 判断某个 NewAPI Token 是否已被某个 Identity Profile 绑定（§11-C §C.1）。
 *
 * 通过 `listIdentityProfiles({ token_id, page: 1, page_size: 1 })` 探测：后端
 * `token_id` 有唯一索引（一 token 一 Profile），只要 total > 0 即视为已存在。
 * 前端用此结果阻止重复创建并引导打开既有 Profile；后端唯一约束继续作为并发最终门禁。
 *
 * `token_id` 非法（<=0 / 非有限数）时直接返回 false，不发请求。
 */
export async function hasExistingProfileForToken(
  tokenId: number
): Promise<boolean> {
  if (!Number.isFinite(tokenId) || tokenId <= 0) return false
  const res = await listIdentityProfiles({ token_id: tokenId, page: 1, page_size: 1 })
  return res.total > 0
}

/**
 * 受控探测：`tokenId` 为合法正数且 `enabled` 时自动查询该 token 是否已绑定 Profile。
 * 供 C.2 创建表单在选中 Token 后展示「该 API Key 已存在 Profile，禁止重复创建」提示。
 */
export function useTokenProfileExists(tokenId: number | null, enabled = true) {
  return useQuery({
    queryKey: ['ai-governance', 'token-profile-exists', tokenId],
    queryFn: () => hasExistingProfileForToken(tokenId as number),
    enabled: enabled && tokenId != null && tokenId > 0,
  })
}
