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

import type { CredentialPurposeType } from '../types'

/**
 * purpose_type 显示文案（§11-B §6）：请求值保枚举原值，仅展示做映射。
 * 文案明确区分「批准用途」与「已验证客户端」，不出现误导性表述。
 */
const PURPOSE_TYPE_LABELS: Record<CredentialPurposeType, string> = {
  DESKTOP_CLIENT: 'Desktop Client',
  IDE: 'IDE',
  SCRIPT: 'Script',
  SERVICE: 'Service',
  OTHER: 'Other',
}

export const CREDENTIAL_PURPOSE_TYPE_KEYS = Object.keys(
  PURPOSE_TYPE_LABELS
) as CredentialPurposeType[]

export function getPurposeTypeLabel(t: TFunction, type: CredentialPurposeType) {
  return t(PURPOSE_TYPE_LABELS[type])
}

export function purposeTypeOptions(t: TFunction) {
  return CREDENTIAL_PURPOSE_TYPE_KEYS.map((type) => ({
    label: getPurposeTypeLabel(t, type),
    value: type,
  }))
}
