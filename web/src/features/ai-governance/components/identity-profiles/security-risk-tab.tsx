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

import { sideDrawerSectionClassName } from '@/components/drawer-layout'
import { formatTimestamp } from '@/lib/format'

import type { GovernanceIdentityProfileDetail } from '../../types'
import { DetailField } from './detail-field'

function YesNo({ value }: { value: boolean }) {
  const { t } = useTranslation()
  return <span>{value ? t('Yes') : t('No')}</span>
}

function QuotaDisplay({ value }: { value: number }) {
  const { t } = useTranslation()
  if (value < 0) return <span>{t('Unlimited')}</span>
  return (
    <span>
      {value.toLocaleString()}
      <span className='text-muted-foreground ml-1 text-xs'>{t('quota')}</span>
    </span>
  )
}

/** 返回 Token Security Status 的 i18n key。1 Enabled / 2 Disabled / 3 Expired /
 * 4 Exhausted / 其他 Unknown —— 绝不得把所有非 1 状态折叠成 Disabled。 */
function tokenStatusKey(status: number): string {
  switch (status) {
    case 1:
      return 'Enabled'
    case 2:
      return 'Disabled'
    case 3:
      return 'Expired'
    case 4:
      return 'Exhausted'
    default:
      return 'Unknown'
  }
}

/**
 * 安全与风险只读页（§11-C §C.3 D）。
 *
 * 只读展示：Token 安全元数据（绝不暴露 Token Key）与弱身份凭证风险姿态。
 * - `rotation_overdue` 文案必须表述为「NewAPI API Key / Credential Rotation Overdue」，
 *   不得写成 HMAC —— 这里的密钥指 NewAPI API Key / 凭证，不是 HMAC 签名密钥。
 * - 全部只读，无任何编辑控件。
 * - `rate_limits.items` 第一阶段恒为空，不得编造明细行。
 */
export function SecurityRiskTab({
  detail,
}: {
  detail: GovernanceIdentityProfileDetail
}) {
  const { t } = useTranslation()
  const { token, risk } = detail

  return (
    <div className='flex flex-col gap-6'>
      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('API Key Security')}</h3>
        <dl className='grid grid-cols-2 gap-4'>
          <DetailField label={t('Status')}>
            <span className='text-sm'>{t(tokenStatusKey(token.status))}</span>
          </DetailField>
          <DetailField label={t('Expired At')}>
            {token.expired_time > 0 ? formatTimestamp(token.expired_time) : t('Never')}
          </DetailField>
          <DetailField label={t('IP Restricted')}>
            <YesNo value={token.ip_restricted} />
          </DetailField>
          <DetailField label={t('Model Restricted')}>
            <YesNo value={token.model_restricted} />
          </DetailField>
          <DetailField label={t('Unlimited Quota')}>
            <YesNo value={token.unlimited} />
          </DetailField>
          <DetailField label={t('Remaining Quota')}>
            <QuotaDisplay value={token.remain_quota} />
          </DetailField>
        </dl>
      </section>

      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Risk Posture')}</h3>
        <dl className='grid grid-cols-2 gap-4'>
          <DetailField label={t('Risk Level')}>
            <span className='font-medium'>{risk.risk_level}</span>
          </DetailField>
          <DetailField label={t('IP Restricted')}>
            <YesNo value={risk.ip_restricted} />
          </DetailField>
          <DetailField label={t('Model Restricted')}>
            <YesNo value={risk.model_restricted} />
          </DetailField>
          <DetailField label={t('Quota Restricted')}>
            <YesNo value={risk.quota_restricted} />
          </DetailField>
          <DetailField label={t('Expiry Configured')}>
            <YesNo value={risk.expiry_configured} />
          </DetailField>
          <DetailField label={t('Rate Limit Enabled')}>
            <YesNo value={risk.rate_limit_enabled} />
          </DetailField>
          <DetailField label={t('Credential Only')}>
            <YesNo value={risk.credential_only} />
          </DetailField>
        </dl>

        <dl className='grid grid-cols-1 gap-4'>
          <DetailField label={t('NewAPI API Key / Credential Rotation Overdue')}>
            <YesNo value={risk.rotation_overdue} />
            {risk.rotation_overdue && risk.rotation_overdue_days != null && (
              <span className='text-muted-foreground ml-2 text-xs'>
                {t('{{n}} days since last rotation', { n: risk.rotation_overdue_days })}
              </span>
            )}
          </DetailField>
        </dl>
      </section>
    </div>
  )
}
