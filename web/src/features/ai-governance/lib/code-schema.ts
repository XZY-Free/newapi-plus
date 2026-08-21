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
 * 治理主数据 code 校验规则（§11-B，前端预校验，后端仍是最终校验边界）。
 *
 * 后端 `service/ai_identity.go` 分两类：
 * - `validateDomainCode`（domain_code / app_code）：小写字母开头，仅
 *   `[a-z0-9._-]`，2~64 字符。→ {@link getDomainCodeSchema}
 * - `validateSimpleCode`（owner_team_code / usage_team_code /
 *   purpose_code / principal_code）：非空、无空白、长度受限（team/purpose 64，
 *   principal 128）。→ {@link getSimpleCodeSchema}
 *
 * 前端校验必须与后端一致，不得对简单 code 误加「小写开头」等更强约束。
 */
export const GOVERNANCE_CODE_MAX_LENGTH = 64
export const GOVERNANCE_CODE_PATTERN = /^[a-z][a-z0-9._-]*$/

/** 简单 code 的统一上限（owner / usage / purpose）。 */
export const SIMPLE_CODE_MAX_LENGTH = 64
/** principal_code 的上限（后端 validateSimpleCode(code, 128)）。 */
export const PRINCIPAL_CODE_MAX_LENGTH = 128

/** domain_code / app_code：小写开头、仅 [a-z0-9._-]、2~64。 */
export function getDomainCodeSchema(t: TFunction) {
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

/** team / purpose / principal code：非空、无空白、长度受限。 */
export function getSimpleCodeSchema(t: TFunction, maxLength: number) {
  return z
    .string()
    .min(1, t('Code is required'))
    .regex(/^\S+$/, t('Code must not contain whitespace'))
    .max(
      maxLength,
      t('Code must be at most {{n}} characters').replace(
        '{{n}}',
        String(maxLength)
      )
    )
}

/**
 * 名称字段（domain / team / principal / purpose / app name）：非空、RuneCount ≤ 128，
 * 与后端 `validateNameLen` 一致（Unicode 字符数，非字节）。
 */
export function getNameSchema(t: TFunction) {
  return z
    .string()
    .min(1, t('Name is required'))
    .max(128, t('Name must be at most 128 characters'))
}
