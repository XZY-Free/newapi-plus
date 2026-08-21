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
import type { ReactNode } from 'react'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'

/**
 * 企业 AI 治理分区的公共页面外壳。
 *
 * 复用 NewAPI `SectionPageLayout`，统一标题与“Root”徽标，供 §11 全部 9 个分区
 * 与治理首页共用，避免各子批各自拼页面骨架。
 */
export function GovernancePageLayout({
  title,
  badge = 'Root',
  actions,
  children,
}: {
  title: ReactNode
  badge?: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{title}</span>
          <Badge variant='outline' className='shrink-0'>
            {badge}
          </Badge>
        </span>
      </SectionPageLayout.Title>
      {actions != null ? <SectionPageLayout.Actions>{actions}</SectionPageLayout.Actions> : null}
      <SectionPageLayout.Content>{children}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
