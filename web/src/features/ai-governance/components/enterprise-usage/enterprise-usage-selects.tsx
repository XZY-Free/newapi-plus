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
  listCredentialPurposes,
  listIdentityProfiles,
  listPrincipals,
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
        listPrincipals({ page, page_size, keyword, enabled: true }).then(
          (res) => ({
            ...res,
            items: res.items.map((p) => ({
              id: p.id,
              principal_name: p.principal_name,
            })),
          })
        )
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.principal_name}
      itemToDescription={(item) => `${t('Principal ID')}: ${item.id}`}
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
        listCredentialPurposes({ page, page_size, keyword, enabled: true }).then(
          (res) => ({
            ...res,
            items: res.items.map((p) => ({
              id: p.id,
              purpose_name: p.purpose_name,
            })),
          })
        )
      }
      itemToValue={(item) => item.id}
      itemToLabel={(item) => item.purpose_name}
      itemToDescription={(item) => `${t('Purpose ID')}: ${item.id}`}
      placeholder={t('Select credential purpose')}
      emptyText={t('No credential purpose found')}
    />
  )
}
