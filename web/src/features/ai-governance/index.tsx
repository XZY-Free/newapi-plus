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

import { GovernancePageLayout } from './components/governance-page-layout'
import {
  AI_GOVERNANCE_DEFAULT_SECTION,
  getGovernanceSectionContent,
  getGovernanceSectionMeta,
  isGovernanceSectionId,
} from './section-registry'

const route = getRouteApi('/_authenticated/ai-governance/$section')

/**
 * 企业 AI 治理分区页（§11）。
 *
 * 由 `/ai-governance/$section` 路由驱动：校验 section 参数后，通过分区注册表
 * 渲染对应分区的页面内容。页面外壳复用 `GovernancePageLayout`。
 */
export function AIGovernance() {
  const { t } = useTranslation()
  const params = route.useParams()

  const sectionId = isGovernanceSectionId(params.section)
    ? params.section
    : AI_GOVERNANCE_DEFAULT_SECTION
  const meta = getGovernanceSectionMeta(sectionId)

  return (
    <GovernancePageLayout title={t(meta.titleKey)}>
      {getGovernanceSectionContent(sectionId, {})}
    </GovernancePageLayout>
  )
}
