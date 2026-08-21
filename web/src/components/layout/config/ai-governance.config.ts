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

import {
  AI_GOVERNANCE_SECTION_META,
  getAIGovernanceSectionNavItems,
  type GovernanceSectionId,
} from '@/features/ai-governance/section-registry'

import type { NavGroup, SidebarView } from '../types'

/**
 * 治理分区在 drill-in 侧边栏中的分组归属。
 */
const SECTION_GROUPS: Array<{
  titleKey: string
  sections: GovernanceSectionId[]
}> = [
  {
    titleKey: 'Governance Master Data',
    sections: [
      'business-domains',
      'usage-teams',
      'principals',
      'credential-purposes',
      'owner-teams',
    ],
  },
  {
    titleKey: 'Identity & Audit',
    sections: ['applications', 'identity-profiles', 'identity-audit'],
  },
  {
    titleKey: 'Usage & Observability',
    sections: ['usage'],
  },
]

/**
 * Sidebar nav groups for the Enterprise AI Governance nested view.
 *
 * 复用分区注册表生成的条目（标题/URL），并叠加各分区的 Lucide 图标。
 */
function getAIGovernanceNavGroups(t: TFunction): NavGroup[] {
  const navItems = getAIGovernanceSectionNavItems(t)
  const titleByUrl = new Map(navItems.map((item) => [item.url, item.title]))

  return SECTION_GROUPS.map((group) => ({
    id: group.titleKey
      .toLowerCase()
      .replaceAll(/[^a-z0-9]+/g, '-')
      .replaceAll(/^-|-$/g, ''),
    title: t(group.titleKey),
    items: group.sections.map((section) => ({
      title: titleByUrl.get(`/ai-governance/${section}`) ?? t(section),
      url: `/ai-governance/${section}`,
      icon: AI_GOVERNANCE_SECTION_META[section].icon,
    })),
  }))
}

/**
 * Nested sidebar view for `/ai-governance/*`.
 *
 * 与 System Settings 相同的 Vercel 式 drill-in：进入企业 AI 治理后，
 * 根导航被治理分区导航替换，并提供返回 Dashboard 的入口。
 */
export const AI_GOVERNANCE_VIEW: SidebarView = {
  id: 'ai-governance',
  pathPattern: /^\/ai-governance(\/|$)/,
  parent: {
    to: '/dashboard/overview',
    label: 'Back to Dashboard',
  },
  getNavGroups: getAIGovernanceNavGroups,
}
