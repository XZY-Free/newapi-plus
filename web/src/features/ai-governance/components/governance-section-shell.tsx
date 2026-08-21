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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import {
  AI_GOVERNANCE_DEFAULT_SECTION,
  AI_GOVERNANCE_SECTION_META,
  getGovernanceSectionMeta,
  isGovernanceSectionId,
} from '../section-registry'

const route = getRouteApi('/_authenticated/ai-governance/$section')

/**
 * 治理分区内容外壳（§11-A 骨架）。
 *
 * 读取当前 `$section` 路由参数，渲染分区标题、说明与作用域提示。
 * 后续子批（§11-B～E）将逐区以完整管理界面替换本外壳的 `CardContent` 主体；
 * 本组件作为 §11-A 的过渡载体，不属于任何分区的最终状态。
 */
export function GovernanceSectionShell() {
  const { t } = useTranslation()
  const params = route.useParams()

  const sectionId = isGovernanceSectionId(params.section)
    ? params.section
    : AI_GOVERNANCE_DEFAULT_SECTION
  const meta = getGovernanceSectionMeta(sectionId)
  const { icon: Icon, descriptionKey } = AI_GOVERNANCE_SECTION_META[sectionId]

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <Icon className='size-4 text-muted-foreground' />
          <span>{t(meta.titleKey)}</span>
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
