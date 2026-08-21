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
import type { TFunction } from 'i18next'
import { z } from 'zod'

/**
 * 治理主数据稳定编码规则（§11-B，前端预校验，后端仍是最终校验边界）。
 *
 * - 小写英文字母开头；
 * - 允许小写字母、数字、`.`、`_`、`-`；
 * - 2~64 字符。
 *
 * 适用于 Business Domain / Usage Team / Owner Team / Principal / Application /
 * Credential Purpose 的 `*_code` 字段。创建后稳定不可改，编辑态由表单置为只读。
 */
export const GOVERNANCE_CODE_MAX_LENGTH = 64

export const GOVERNANCE_CODE_PATTERN = /^[a-z][a-z0-9._-]*$/

export function getGovernanceCodeSchema(t: TFunction) {
  return z
    .string()
    .min(2, t('Code must be at least 2 characters'))
    .max(
      GOVERNANCE_CODE_MAX_LENGTH,
      t('Code must be at most 64 characters')
    )
    .regex(
      GOVERNANCE_CODE_PATTERN,
      t(
        'Code must start with a lowercase letter and contain only lowercase letters, numbers, dots, underscores or hyphens'
      )
    )
}
