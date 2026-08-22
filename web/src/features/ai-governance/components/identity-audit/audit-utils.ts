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
import type { IdentityAuditResult } from '../../types'

/**
 * result 徽标颜色：UNVERIFIED=降级放行（warning），REJECTED=enforce 拒绝（danger）。
 */
export function auditResultVariant(
  result: IdentityAuditResult
): 'warning' | 'danger' {
  return result === 'REJECTED' ? 'danger' : 'warning'
}

/**
 * 后端 GetIdentityAuditEvents 实际落库的 reason_code 全集（constant 枚举 + 强身份
 * 失败 reason）。仅作 result/reason 筛选候选；事件里出现的任何其他 code 也照常展示。
 */
export const AUDIT_REASON_CODES: readonly string[] = [
  'PROFILE_REQUIRED',
  'PROFILE_DISABLED',
  'STORE_UNAVAILABLE',
  'ATTRIBUTION_MODE_INVALID',
  'IDENTITY_MODE_INVALID',
  'TARGET_INVALID',
  'ASSURANCE_INVALID',
  'PRINCIPAL_REQUIRED',
  'PURPOSE_REQUIRED',
  'USAGE_TEAM_INVALID',
  'APP_NOT_BOUND',
  'PRINCIPAL_DISABLED',
  'PURPOSE_DISABLED',
  'APP_DISABLED',
  'CONTEXT_REQUIRED',
  'CONTEXT_INVALID',
  'CONTEXT_TOO_LARGE',
  'NONCE_INVALID',
  'TIMESTAMP_INVALID',
  'KEY_INVALID',
  'SIGNATURE_INVALID',
  'REPLAY_DETECTED',
  'REPLAY_STORE_UNAVAILABLE',
  'APP_NOT_ALLOWED',
  'HYBRID_APP_MISMATCH',
  'CREDENTIAL_RATE_LIMIT_EXCEEDED',
  'CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE',
]
