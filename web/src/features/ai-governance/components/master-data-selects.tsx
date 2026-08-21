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

import {
  listBusinessDomains,
  listOwnerTeams,
  listUsageTeams,
} from '../api'
import { MasterDataSelect } from './master-data-select'

type TeamSelectProps = {
  id?: string
  value: number | null
  onChange: (value: number | null) => void
  defaultLabel?: string
  disabled?: boolean
  className?: string
}

/**
 * 业务领域选择器（§11-B §八）。
 * 打开即从第 1 页加载，输入关键词服务端搜索，列表底部可翻页加载更多。
 */
export function BusinessDomainSelect(props: TeamSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'business-domain-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listBusinessDomains({ page, page_size, keyword, enabled: true })
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.domain_name}
      placeholder={t('Select business domain')}
      emptyText={t('No business domain found')}
    />
  )
}

/**
 * 使用团队选择器（§11-B §八）。
 * Usage Team = 凭证使用人所属团队，与 Application Owner Team 不是一回事。
 */
export function UsageTeamSelect(props: TeamSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-team-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listUsageTeams({ page, page_size, keyword, enabled: true })
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.team_name}
      placeholder={t('Select usage team')}
      emptyText={t('No usage team found')}
    />
  )
}

/**
 * 应用负责团队选择器（§11-B §八）。
 * Application Owner Team = AI 应用建设/维护/运营负责团队，不得与 Usage Team 混用。
 */
export function OwnerTeamSelect(props: TeamSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'owner-team-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listOwnerTeams({ page, page_size, keyword, enabled: true })
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.team_name}
      placeholder={t('Select owner team')}
      emptyText={t('No owner team found')}
    />
  )
}
