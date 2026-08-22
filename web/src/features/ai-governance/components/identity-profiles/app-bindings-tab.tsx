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
import { useMutation } from '@tanstack/react-query'
import { Info } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { sideDrawerSectionClassName } from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { handleServerError } from '@/lib/handle-server-error'

import { replaceIdentityProfileAppBindings } from '../../api'
import { findIdentityProfileModeConfig } from '../../lib/identity-profile-mode'
import type { GovernanceIdentityProfileDetail } from '../../types'
import { ApplicationMultiSelect } from './profile-selects'

/**
 * App Bindings 页（§11-C §C.3 C）。
 *
 * - 展示 `detail.bindings`（app_name / app_code / business_domain / owner_team / enabled）。
 * - 编辑复用唯一的 `ApplicationMultiSelect`（与 Create 共用，不建立第二套选择器），
 *   经 `selectedMeta` 让历史已停用的已绑定应用仍以真实 name/code + Disabled 可见、可移除，
 *   绝不静默消失。
 * - 保存走**整体替换** `replaceIdentityProfileAppBindings(profileId, { app_ids })`，
 *   不做增量 diff。新候选仅 enabled=true（由选择器 fetch 保证）。
 * - App 数量上限/下限复用 {@link IDENTITY_PROFILE_MODES} 单一事实源。
 */
export function AppBindingsTab({
  detail,
  onChanged,
}: {
  detail: GovernanceIdentityProfileDetail
  onChanged: () => void
}) {
  const { t } = useTranslation()
  const { profile, bindings } = detail
  const config = findIdentityProfileModeConfig(
    profile.identity_mode,
    profile.attribution_target_type,
    profile.identity_assurance
  )

  const [appIds, setAppIds] = useState<number[]>(() =>
    bindings.map((b) => b.app_id)
  )
  // 与 create 共用同一个选择器：已选但不在 enabled 候选列表中的（历史停用）绑定应用，
  // 经 selectedMeta 传入以真实名称/code + Disabled 渲染。
  const selectedMeta: Record<number, { app_name: string; app_code: string; enabled: boolean }> =
    {}
  for (const b of bindings) {
    selectedMeta[b.app_id] = {
      app_name: b.app_name,
      app_code: b.app_code,
      enabled: b.enabled,
    }
  }

  // 详情重拉（如整体替换后）时重置本地集合，避免残留已被后端清除的 id。
  useEffect(() => {
    setAppIds(bindings.map((b) => b.app_id))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detail])

  const mutation = useMutation({
    mutationFn: () =>
      replaceIdentityProfileAppBindings(profile.id, { app_ids: appIds }),
    onSuccess: () => {
      toast.success(t('App bindings updated'))
      onChanged()
    },
    onError: handleServerError,
  })

  const usesApps = config?.usesApps ?? false
  const withinCardinality =
    !usesApps ||
    (appIds.length >= (config?.appMin ?? 0) &&
      (config?.appMax === Number.POSITIVE_INFINITY ||
        appIds.length <= (config?.appMax ?? Infinity)))

  if (!usesApps) {
    return (
      <div className='flex flex-col gap-6'>
        <section className={sideDrawerSectionClassName()}>
          <h3 className='text-sm font-semibold'>{t('App Bindings')}</h3>
          <p className='flex items-start gap-1.5 text-xs leading-5 text-muted-foreground'>
            <Info className='mt-0.5 size-3.5 shrink-0' />
            {t('This identity mode does not bind applications.')}
          </p>
        </section>
      </div>
    )
  }

  const cardinalityText =
    config?.appMin === config?.appMax
      ? t('Select exactly {{n}} application', { n: config?.appMin ?? 0 })
      : t('Select at least {{n}} application', { n: config?.appMin ?? 1 })

  return (
    <div className='flex flex-col gap-6'>
      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Bound Applications')}</h3>
        <ApplicationMultiSelect
          value={appIds}
          onChange={setAppIds}
          selectedMeta={selectedMeta}
        />
        <p className='text-muted-foreground text-xs leading-5'>
          {cardinalityText} ·{' '}
          {t('App bindings are replaced as a whole set on save. New candidates must be enabled.')}
        </p>
      </section>

      {bindings.length > 0 && (
        <section className={sideDrawerSectionClassName()}>
          <h3 className='text-sm font-semibold'>{t('Current Bindings')}</h3>
          <ul className='flex flex-col gap-2'>
            {bindings.map((b) => (
              <li
                key={b.id}
                className='flex items-center justify-between gap-3 rounded-lg border bg-muted/30 px-3 py-2'
              >
                <div className='flex min-w-0 flex-col'>
                  <span className='truncate text-sm font-medium'>
                    {b.app_name}
                    <span className='text-muted-foreground ml-1 text-xs'>
                      ({b.app_code})
                    </span>
                  </span>
                  <span className='text-muted-foreground truncate text-xs'>
                    {[b.business_domain_name, b.owner_team_name]
                      .filter(Boolean)
                      .join(' · ') || '—'}
                  </span>
                </div>
                <span
                  className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] uppercase ${
                    b.enabled
                      ? 'bg-emerald-500/10 text-emerald-600'
                      : 'bg-muted text-muted-foreground'
                  }`}
                >
                  {b.enabled ? t('Enabled') : t('Disabled')}
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className='flex justify-end'>
        <Button
          type='button'
          onClick={() => mutation.mutate()}
          disabled={!withinCardinality || mutation.isPending}
        >
          {mutation.isPending ? t('Saving...') : t('Save changes')}
        </Button>
      </div>
    </div>
  )
}
