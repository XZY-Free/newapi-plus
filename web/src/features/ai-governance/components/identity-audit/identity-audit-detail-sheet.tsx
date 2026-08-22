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
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
  sideDrawerSectionClassName,
} from '@/components/drawer-layout'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { formatTimestamp } from '@/lib/format'

import type { GovernanceAuditEvent } from '../../types'
import { DetailField } from '../identity-profiles/detail-field'
import { auditResultVariant } from './audit-utils'
import { StatusBadge } from '@/components/status-badge'

/**
 * 身份审计事件详情（§11-D）。后端仅有列表接口，事件为自包含、已反规范化的快照，
 * 故详情直接从所选行渲染，不做二次拉取，也不伪造任何后端没有的字段。
 *
 * 严格区分两类事实，绝不混在一起：
 * - **Profile Snapshot（Profile 配置事实）**：identity_mode / identity_assurance /
 *   caller_id / principal_id / credential_purpose_id —— 这是请求命中的 Profile 在
 *   时刻点的配置快照，不代表本次请求的客户端验证成功。
 * - **Verification Outcome（单次请求运行时事实）**：result / reason_code /
 *   claimed_root_app_id —— 本次请求的身份验证结果与原因、客户端声称的 root_app。
 *
 * 后端审计事件**没有** client_verified 字段，前端绝不伪造该事实。
 */
export function IdentityAuditDetailSheet({
  event,
  open,
  onOpenChange,
}: {
  event: GovernanceAuditEvent | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Event Details')}</SheetTitle>
          <SheetDescription>
            {t('Request context, profile snapshot and verification outcome for a single identity audit event.')}
          </SheetDescription>
        </SheetHeader>

        {event != null && (
          <div className='flex flex-col gap-6 overflow-y-auto p-4'>
            <section className={sideDrawerSectionClassName()}>
              <h3 className='text-sm font-semibold'>{t('Request Context')}</h3>
              <dl className='grid grid-cols-2 gap-x-4 gap-y-3'>
                <DetailField label={t('Request ID')}>
                  <code className='break-all font-mono text-xs'>{event.request_id}</code>
                </DetailField>
                <DetailField label={t('Time')}>
                  <span className='font-mono'>{formatTimestamp(event.created_at)}</span>
                </DetailField>
                <DetailField label={t('HTTP Method')}>
                  <span className='font-mono'>{event.http_method}</span>
                </DetailField>
                <DetailField label={t('Client IP')}>
                  <span className='font-mono'>{event.client_ip}</span>
                </DetailField>
                <div className='col-span-2 flex flex-col gap-0.5'>
                  <dt className='text-muted-foreground text-xs'>{t('Request Path')}</dt>
                  <dd className='break-all font-mono text-xs'>{event.request_path}</dd>
                </div>
              </dl>
            </section>

            <section className={sideDrawerSectionClassName()}>
              <h3 className='text-sm font-semibold'>{t('Profile Snapshot')}</h3>
              <p className='text-muted-foreground text-xs leading-5'>
                {t('The identity mode, assurance and caller the resolved profile was configured with at request time. These are profile configuration facts, not proof of client verification.')}
              </p>
              <dl className='grid grid-cols-2 gap-x-4 gap-y-3'>
                <DetailField label={t('Token ID')}>
                  <span className='font-mono'>{event.token_id > 0 ? event.token_id : '—'}</span>
                </DetailField>
                <DetailField label={t('Profile ID')}>
                  <span className='font-mono'>{event.profile_id > 0 ? event.profile_id : '—'}</span>
                </DetailField>
                <DetailField label={t('Caller ID')}>
                  <span className='break-all font-mono text-xs'>{event.caller_id || '—'}</span>
                </DetailField>
                <DetailField label={t('Principal ID')}>
                  <span className='font-mono'>{event.principal_id > 0 ? event.principal_id : '—'}</span>
                </DetailField>
                <DetailField label={t('Credential Purpose ID')}>
                  <span className='font-mono'>{event.credential_purpose_id > 0 ? event.credential_purpose_id : '—'}</span>
                </DetailField>
                <DetailField label={t('Identity Mode')}>
                  <span className='break-all font-mono text-xs'>{event.identity_mode || '—'}</span>
                </DetailField>
                <DetailField label={t('Identity Assurance')}>
                  <span className='break-all font-mono text-xs'>{event.identity_assurance || '—'}</span>
                </DetailField>
              </dl>
            </section>

            <section className={sideDrawerSectionClassName()}>
              <h3 className='text-sm font-semibold'>{t('Verification Outcome')}</h3>
              <p className='text-muted-foreground text-xs leading-5'>
                {t('The single-request runtime outcome: UNVERIFIED means the request continued with a degraded or unverified identity, REJECTED means it was blocked by identity or credential governance. It also records the root app the client claimed. The backend audit event has no client_verified field.')}
              </p>
              <dl className='grid grid-cols-2 gap-x-4 gap-y-3'>
                <DetailField label={t('Result')}>
                  <StatusBadge
                    label={event.result}
                    variant={auditResultVariant(event.result)}
                    copyable={false}
                  />
                </DetailField>
                <DetailField label={t('Reason Code')}>
                  <span className='break-all font-mono text-xs'>{event.reason_code || '—'}</span>
                </DetailField>
                <div className='col-span-2 flex flex-col gap-0.5'>
                  <dt className='text-muted-foreground text-xs'>{t('Claimed Root App')}</dt>
                  <dd className='break-all font-mono text-xs'>
                    {event.claimed_root_app_id || '—'}
                  </dd>
                </div>
              </dl>
            </section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
