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
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { handleServerError } from '@/lib/handle-server-error'

import {
  replaceIdentityProfileAppBindings,
  updateIdentityProfile,
} from '../../api'
import {
  buildIdentityProfileReconfigurePlan,
  findIdentityProfileModeConfig,
  getIdentityProfileFormSchema,
  IDENTITY_PROFILE_MODES,
  identityProfileReconfigureDefaults,
  type IdentityProfileFormValues,
} from '../../lib/identity-profile-mode'
import type { GovernanceIdentityProfileDetail } from '../../types'
import { DetailField } from './detail-field'
import { ApplicationMultiSelect, PrincipalSelect, PurposeSelect } from './profile-selects'
import { useSelectedApplicationMeta } from './use-selected-application-meta'

const PRESET_VALUE_SEPARATOR = '|'

function presetValueOf(config: {
  identity_mode: string
  attribution_target_type: string
  identity_assurance: string
}): string {
  return [config.identity_mode, config.attribution_target_type, config.identity_assurance].join(
    PRESET_VALUE_SEPARATOR
  )
}

/** 四种合法身份模式预置（复用 IDENTITY_PROFILE_MODES 单一事实源，不建立第二套矩阵）。 */
const PRESET_OPTIONS = IDENTITY_PROFILE_MODES.map((config) => ({
  value: presetValueOf(config),
  config,
}))

function assuranceHelpKey(assurance: string): string {
  switch (assurance) {
    case 'SIGNED_CONTEXT':
      return 'The caller must present a trusted signed context'
    case 'HYBRID_VERIFIED_CONTEXT':
      return 'Fixed application attribution plus signed runtime context'
    default:
      return 'The API key credential is attributable; the client itself is not verified'
  }
}

/**
 * 重配置身份模式（§11-C §C.3 E）。
 *
 * 与普通 Edit 严格分离：普通 Edit 禁止修改核心三元组（identity_mode /
 * attribution_target_type / identity_assurance）与 App Bindings；本组件是**唯一**
 * 允许切换核心三元组的入口，且仅当 `profile.enabled=false` 时可用（启用中的 Profile
 * 必须先停用，UI 绝不自动停用）。
 *
 * 固定流程（由本组件编排，后端无原子性改动）：
 *   1. Replace App Bindings（整体替换 `{ app_ids }`）
 *   2. Update Profile 核心三元组 + 显式清理/设置各模式的隐藏字段
 *   3. 重新拉取详情（onSuccess）
 * 任一步失败：Profile 保持停用、展示真实错误、强制重拉详情、不自动回滚、不自动启用。
 */
export function ReconfigureIdentityModeSheet({
  detail,
  open,
  onOpenChange,
  onSuccess,
  onFailed,
}: {
  detail: GovernanceIdentityProfileDetail
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
  /** 任一步失败后触发：强制重新 getIdentityProfile + 刷新列表（P1-3）。 */
  onFailed: () => void
}) {
  const { t } = useTranslation()
  const { profile, principal, purpose, bindings } = detail

  const form = useForm<IdentityProfileFormValues>({
    resolver: zodResolver(getIdentityProfileFormSchema(t, { edit: false })),
    defaultValues: identityProfileReconfigureDefaults({ profile, bindings }),
  })

  const [isSubmitting, setIsSubmitting] = useState(false)

  const [identityMode, attributionTarget, assurance] = form.watch([
    'identity_mode',
    'attribution_target_type',
    'identity_assurance',
  ])
  const config = findIdentityProfileModeConfig(
    identityMode,
    attributionTarget,
    assurance
  )

  // selectedMeta 用 Application 当前 enabled（P1-4），非 binding 行 enabled。
  const selectedMeta = useSelectedApplicationMeta(bindings)

  const handlePresetChange = (composite: string) => {
    const preset = PRESET_OPTIONS.find((o) => o.value === composite)?.config
    if (!preset) return
    form.setValue('identity_mode', preset.identity_mode)
    form.setValue('attribution_target_type', preset.attribution_target_type)
    form.setValue('identity_assurance', preset.identity_assurance)
    // 模式联动必须清理隐藏字段，绝不残留旧模式的 Principal/Purpose/Caller/App。
    if (!preset.usesPrincipal) form.setValue('principal_id', null)
    if (!preset.usesPurpose) form.setValue('credential_purpose_id', null)
    if (!preset.usesCaller) {
      form.setValue('caller_id', '')
      form.setValue('caller_name', '')
    }
    if (!preset.usesApps) form.setValue('app_ids', [])
    form.clearErrors([
      'principal_id',
      'credential_purpose_id',
      'caller_id',
      'caller_name',
      'app_ids',
    ])
  }

  const onSubmit = async (values: IdentityProfileFormValues) => {
    const cfg = findIdentityProfileModeConfig(
      values.identity_mode,
      values.attribution_target_type,
      values.identity_assurance
    )
    if (!cfg) return
    const { app_ids, profilePatch } = buildIdentityProfileReconfigurePlan(values, cfg)

    setIsSubmitting(true)
    try {
      // 步骤 1：先整体替换 App Bindings。
      await replaceIdentityProfileAppBindings(profile.id, { app_ids })
      // 步骤 2：再更新 Profile 核心三元组（含隐藏字段显式清理）。绝不含 enabled。
      await updateIdentityProfile(profile.id, profilePatch)
      toast.success(t('Identity mode reconfigured'))
      onSuccess()
    } catch (error) {
      // 任一步失败：Profile 保持停用，透出真实错误，不自动回滚/启用，
      // 并强制重新拉取真实详情（此时服务器 Bindings 可能已改变，P1-3）。
      handleServerError(error)
      onFailed()
      onOpenChange(false)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Reconfigure Identity Mode')}</SheetTitle>
          <SheetDescription>
            {t('Change the identity mode, attribution target and assurance for this profile. The profile is currently disabled.')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='identity-profile-reconfigure-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <fieldset disabled={isSubmitting} className='contents'>
              <SideDrawerSection>
                <h3 className='text-sm font-semibold'>{t('Bound API Key')}</h3>
                <dl className='grid grid-cols-2 gap-4'>
                  <DetailField label={t('API Key')}>
                    <span className='font-medium'>{detail.token.token_name}</span>
                  </DetailField>
                  <DetailField label={t('Token ID')}>
                    {detail.token.token_id}
                  </DetailField>
                </dl>
              </SideDrawerSection>

              <SideDrawerSection>
                <FormField
                  control={form.control}
                  name='identity_mode'
                  render={() => (
                    <FormItem>
                      <FormLabel>{t('Identity Configuration')}</FormLabel>
                      <FormControl>
                        <RadioGroup
                          value={presetValueOf({ identity_mode: identityMode, attribution_target_type: attributionTarget, identity_assurance: assurance })}
                          onValueChange={handlePresetChange}
                        >
                          {PRESET_OPTIONS.map(({ value, config: preset }) => (
                            <label
                              key={value}
                              className='flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-2.5 has-data-checked:border-primary has-data-checked:bg-primary/5'
                            >
                              <RadioGroupItem value={value} className='mt-0.5' />
                              <span className='flex min-w-0 flex-col gap-0.5'>
                                <span className='text-sm font-medium'>
                                  {preset.identity_mode} · {preset.attribution_target_type} · {preset.identity_assurance}
                                </span>
                                <span className='text-muted-foreground text-xs'>
                                  {t(assuranceHelpKey(preset.identity_assurance))}
                                </span>
                              </span>
                            </label>
                          ))}
                        </RadioGroup>
                      </FormControl>
                      <FormDescription>
                        {t('Reconfiguring clears hidden fields from the previous mode. The assurance level is set by the combination, not chosen freely.')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SideDrawerSection>

              {config?.usesPrincipal && (
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='principal_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Principal')}</FormLabel>
                        <FormControl>
                          <PrincipalSelect
                            value={field.value}
                            onChange={field.onChange}
                            defaultLabel={principal?.principal_name}
                            defaultDescription={principal?.principal_code}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='credential_purpose_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Credential Purpose')}</FormLabel>
                        <FormControl>
                          <PurposeSelect
                            value={field.value}
                            onChange={field.onChange}
                            defaultLabel={purpose?.credential_purpose_name}
                            defaultDescription={purpose?.credential_purpose_code}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              )}

              {config?.usesCaller && (
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='caller_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Caller ID')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('e.g. platform, bot-svc')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='caller_name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Caller Name')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('Optional')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              )}

              {config?.usesApps && (
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='app_ids'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Applications')}</FormLabel>
                        <FormControl>
                          <ApplicationMultiSelect
                            value={field.value}
                            onChange={field.onChange}
                            selectedMeta={selectedMeta}
                          />
                        </FormControl>
                        <FormDescription>
                          {config.appMin === config.appMax
                            ? t('Select exactly {{n}} application', { n: config.appMin })
                            : t('Select at least {{n}} application', { n: config.appMin })}
                          {' · '}
                          {t('App bindings are replaced as a whole set.')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              )}
            </fieldset>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button form='identity-profile-reconfigure-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : t('Reconfigure')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
