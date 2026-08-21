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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  AI_GOVERNANCE_SECTION_IDS,
  AI_GOVERNANCE_SECTION_META,
  getGovernanceSectionMeta,
  type GovernanceSectionId,
} from '../section-registry'

/**
 * 企业 AI 治理首页 / Overview。
 *
 * 仅做导航与说明：以九宫格呈现 9 个治理分区入口，点击进入
 * `/ai-governance/$section`。不提前实现任何 §11-B～E 业务能力，
 * 也不新增第 10 个治理分区。
 */
export function GovernanceOverview() {
  const { t } = useTranslation()

  return (
    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'>
      {AI_GOVERNANCE_SECTION_IDS.map((section: GovernanceSectionId) => {
        const meta = getGovernanceSectionMeta(section)
        const card = AI_GOVERNANCE_SECTION_META[section]
        const Icon = card.icon
        return (
          <Link
            key={section}
            to='/ai-governance/$section'
            params={{ section }}
            className='transition-colors'
          >
            <Card className='h-full hover:border-foreground/30'>
              <CardHeader>
                <CardTitle className='flex items-center gap-2 text-base'>
                  <Icon className='size-4 text-muted-foreground' />
                  <span>{t(meta.titleKey)}</span>
                </CardTitle>
                <CardDescription>{t(card.descriptionKey)}</CardDescription>
              </CardHeader>
            </Card>
          </Link>
        )
      })}
    </div>
  )
}
