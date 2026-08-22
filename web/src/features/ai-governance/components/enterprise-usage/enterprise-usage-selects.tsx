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
  listCredentialPurposes,
  listIdentityProfiles,
  listOwnerTeams,
  listPrincipals,
  listUsageTeams,
} from '../../api'
import { MasterDataSelect } from '../master-data-select'

export type UsageFilterSelectProps = {
  id?: string
  value: number | null
  onChange: (value: number | null) => void
  defaultLabel?: string
  disabled?: boolean
  className?: string
}

/** Identity Profile 选择器：按 profile 过滤企业用量（§12 profile_id）。 */
export function UsageProfileSelect(props: UsageFilterSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<{
      id: number
      caller_name: string
      token_name: string
    }>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-profile-options']}
      fetchPage={async ({ page, page_size, keyword }) => {
        const res = await listIdentityProfiles({
          page,
          page_size,
          keyword: keyword?.trim() || undefined,
        })
        return {
          ...res,
          items: res.items.map((d) => ({
            id: d.profile.id,
            caller_name: d.profile.caller_name,
            token_name: d.token.token_name,
          })),
        }
      }}
      itemToValue={(item) => item.id}
      itemToLabel={(item) =>
        item.caller_name || item.token_name || `${item.id}`
      }
      itemToDescription={(item) => `${t('Profile ID')}: ${item.id}`}
      placeholder={t('Select identity profile')}
      emptyText={t('No identity profile found')}
    />
  )
}

/** Principal 选择器：按弱身份个人主体过滤（§12 principal_id）。 */
export function UsagePrincipalSelect(props: UsageFilterSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<{
      id: number
      principal_code: string
      principal_name: string
    }>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-principal-options']}
      fetchPage={({ page, page_size, keyword }) =>
        // E.2 P1-G：历史分析不限 enabled，允许选已停用但历史仍在用的主体。
        listPrincipals({ page, page_size, keyword }).then((res) => ({
          ...res,
          items: res.items.map((p) => ({
            id: p.id,
            principal_code: p.principal_code,
            principal_name: p.principal_name,
          })),
        }))
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.principal_name}
      itemToDescription={(item) => item.principal_code}
      placeholder={t('Select principal')}
      emptyText={t('No principal found')}
    />
  )
}

/** Credential Purpose 选择器：按用途过滤（§12 credential_purpose_id）。 */
export function UsagePurposeSelect(props: UsageFilterSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<{
      id: number
      purpose_code: string
      purpose_name: string
    }>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-purpose-options']}
      fetchPage={({ page, page_size, keyword }) =>
        // E.2 P1-G：历史分析不限 enabled，允许选已停用但历史仍在用的用途。
        listCredentialPurposes({ page, page_size, keyword }).then((res) => ({
          ...res,
          items: res.items.map((p) => ({
            id: p.id,
            purpose_code: p.purpose_code,
            purpose_name: p.purpose_name,
          })),
        }))
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.purpose_name}
      itemToDescription={(item) => item.purpose_code}
      placeholder={t('Select credential purpose')}
      emptyText={t('No credential purpose found')}
    />
  )
}

/**
 * 业务领域选择器（企业用量历史分析专用，E.2 P1-G）。
 * 与 §11-B 冻结构建用 `BusinessDomainSelect`（enabled:true）不同，历史分析必须
 * 能选已停用但历史仍在用的领域，故不限制 enabled。
 */
export function UsageBusinessDomainSelect(props: UsageFilterSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<{
      id: number
      domain_code: string
      domain_name: string
    }>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-business-domain-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listBusinessDomains({ page, page_size, keyword }).then((res) => ({
          ...res,
          items: res.items.map((d) => ({
            id: d.id,
            domain_code: d.domain_code,
            domain_name: d.domain_name,
          })),
        }))
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.domain_name}
      itemToDescription={(item) => item.domain_code}
      placeholder={t('Select business domain')}
      emptyText={t('No business domain found')}
    />
  )
}

/**
 * 使用团队选择器（企业用量历史分析专用，E.2 P1-G）。
 * Usage Team = 凭证使用人所属团队，与 Application Owner Team 不是一回事。
 */
export function UsageUsageTeamSelect(props: UsageFilterSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<{
      id: number
      team_code: string
      team_name: string
    }>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-usage-team-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listUsageTeams({ page, page_size, keyword }).then((res) => ({
          ...res,
          items: res.items.map((team) => ({
            id: team.id,
            team_code: team.team_code,
            team_name: team.team_name,
          })),
        }))
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.team_name}
      itemToDescription={(item) => item.team_code}
      placeholder={t('Select usage team')}
      emptyText={t('No usage team found')}
    />
  )
}

/**
 * 应用负责团队选择器（企业用量历史分析专用，E.2 P1-G）。
 * Application Owner Team = AI 应用建设/维护/运营负责团队。
 */
export function UsageOwnerTeamSelect(props: UsageFilterSelectProps) {
  const { t } = useTranslation()
  return (
    <MasterDataSelect<{
      id: number
      team_code: string
      team_name: string
    }>
      id={props.id}
      value={props.value}
      onChange={props.onChange}
      defaultLabel={props.defaultLabel}
      disabled={props.disabled}
      className={props.className}
      queryKey={['ai-governance', 'usage-owner-team-options']}
      fetchPage={({ page, page_size, keyword }) =>
        listOwnerTeams({ page, page_size, keyword }).then((res) => ({
          ...res,
          items: res.items.map((team) => ({
            id: team.id,
            team_code: team.team_code,
            team_name: team.team_name,
          })),
        }))
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.team_name}
      itemToDescription={(item) => item.team_code}
      placeholder={t('Select owner team')}
      emptyText={t('No owner team found')}
    />
  )
}
