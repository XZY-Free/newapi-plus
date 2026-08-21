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
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

/**
 * 治理分区内容外壳（§11-A 骨架）。
 *
 * 纯展示组件：不解析路由、不导入分区注册表、不自行解析 section。
 * 标题/说明/图标全部通过 Props 传入，由 `section-registry`（唯一分区事实源）
 * 在 `build` 时选择并注入。
 *
 * 后续子批（§11-B～E）将逐区以完整管理界面替换本外壳的 `CardContent` 主体；
 * 本组件作为 §11-A 的过渡载体，不属于任何分区的最终状态。
 */
export function GovernanceSectionShell({
  titleKey,
  descriptionKey,
  icon: Icon,
}: {
  titleKey: string
  descriptionKey: string
  icon: LucideIcon
}) {
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Icon className='size-4 text-muted-foreground' />
          <span>{t(titleKey)}</span>
        </CardTitle>
        <CardDescription>{t(descriptionKey)}</CardDescription>
      </CardHeader>
      <CardContent>
        <p className='text-sm text-muted-foreground'>
          {t(
            'This governance section is delivered by a later step of the enterprise AI governance rollout'
          )}
        </p>
      </CardContent>
    </Card>
  )
}
