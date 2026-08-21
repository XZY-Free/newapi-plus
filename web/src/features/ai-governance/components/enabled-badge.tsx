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

import { StatusBadge } from '@/components/status-badge'

/**
 * 治理主数据统一启停状态徽标（§11-B §10 §13）。
 *
 * 六个主数据分区共用同一种 enabled 展示语义：启用 = success，停用 = neutral。
 */
export function EnabledBadge({ enabled }: { enabled: boolean }) {
  const { t } = useTranslation()
  return enabled ? (
    <StatusBadge label={t('Enabled')} variant='success' copyable={false} />
  ) : (
    <StatusBadge label={t('Disabled')} variant='neutral' copyable={false} />
  )
}
