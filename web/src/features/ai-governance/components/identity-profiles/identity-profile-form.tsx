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
import { AlertCircle, ShieldCheck } from 'lucide-react'
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
import {
  RadioGroup,
  RadioGroupItem,
} from '@/components/ui/radio-group'
import {
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createIdentityProfile,
  updateIdentityProfile,
} from '../../api'
import {
  buildIdentityProfileCreatePayload,
  buildIdentityProfileEditDelta,
  ENVIRONMENT_DEFAULT,
  findIdentityProfileModeConfig,
  getIdentityProfileFormSchema,
  IDENTITY_PROFILE_MODES,
  identityProfileEditDefaults,
  identityProfileFormDefaults,
  type IdentityProfileFormValues,
} from '../../lib/identity-profile-mode'
import { useTokenProfileExists } from '../../lib/token-profile-exists'
import type { GovernanceIdentityProfileDetail } from '../../types'
import { TokenSelect } from '../token-select'
import {
  ApplicationMultiSelect,
  PrincipalSelect,
  PurposeSelect,
} from './profile-selects'

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

/**
 * 身份模式四项合法预置（§11-C §C.2 三）。单选预置原子地设置三元组，
 * 从根上杜绝第五种组合；`identity_assurance` 由合法组合约束，不可自由选择。
 */
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

type IdentityProfileFormProps = {
  mode: 'create' | 'edit'
  /** Edit 时提供聚合详情以回填默认值与只读展示；Create 时为 null。 */
  currentDetail?: GovernanceIdentityProfileDetail | null
  /** 创建/更新成功后的回调（页面负责关闭抽屉 + 刷新列表 + 失效 dup 缓存）。 */
  onSuccess?: () => void
}

/**
 * Identity Profile 创建 / 当前模式编辑共用表单（§11-C §C.2 二）。
 *
 * - Create / 普通 Edit / C.3 详情页共用这一套 Form + Mode Rules + Schema + Payload Builder。
 * - 模式规则唯一事实源见 `lib/identity-profile-mode.ts`，本组件不散落 if STATIC 分支。
 * - Create 带 Token 重复探测四态（idle/querying/error/exists/free）；
 * - Edit 核心三元组与 token_id 只读，仅当前模式内部字段可编辑，Delta Payload 只发变化字段。
 * - App Bindings 仅 Create 设置；普通 Edit 绝不调用 replaceIdentityProfileAppBindings。
 */
export function IdentityProfileForm({
  mode,
  currentDetail,
  onSuccess,
}: IdentityProfileFormProps) {
  const { t } = useTranslation()
  const isEdit = mode === 'edit'
  const currentProfile = currentDetail?.profile

  const form = useForm<IdentityProfileFormValues>({
    resolver: zodResolver(
      getIdentityProfileFormSchema(t, { edit: isEdit })
    ),
    defaultValues: isEdit && currentProfile
      ? identityProfileEditDefaults(currentProfile)
      : identityProfileFormDefaults(),
  })

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

  const [isSubmitting, setIsSubmitting] = useState(false)

  // Token 重复探测（§11-C §C.2 一 / 十五）：前端探测只是 UX，后端唯一约束是最终门禁。
  const tokenId = form.watch('token_id')
  const dupQuery = useTokenProfileExists(tokenId, mode === 'create')

  const duplicateState = (() => {
    if (mode !== 'create') return 'none'
    if (tokenId == null) return 'idle'
    if (dupQuery.status === 'pending') return 'querying'
    if (dupQuery.isError) return 'error'
    return dupQuery.data ? 'exists' : 'free'
  })()
  const submitBlocked =
    isSubmitting || (mode === 'create' && duplicateState !== 'free')

  const handlePresetChange = (composite: string) => {
    const preset = PRESET_OPTIONS.find((o) => o.value === composite)?.config
    if (!preset) return
    form.setValue('identity_mode', preset.identity_mode)
    form.setValue('attribution_target_type', preset.attribution_target_type)
    form.setValue('identity_assurance', preset.identity_assurance)
    // 模式联动必须清理隐藏字段（Update 是 patch 语义，绝不残留旧 Principal/Purpose/Caller）。
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
    const cfg =
      findIdentityProfileModeConfig(
        values.identity_mode,
        values.attribution_target_type,
        values.identity_assurance
      )
    if (!cfg) return
    setIsSubmitting(true)
    try {
      if (mode === 'create') {
        if (duplicateState !== 'free') return
        await createIdentityProfile(buildIdentityProfileCreatePayload(values, cfg))
        toast.success(t('Identity profile created'))
      } else if (currentProfile) {
        const delta = buildIdentityProfileEditDelta(values, currentProfile, cfg)
        if (Object.keys(delta).length === 0) {
          onSuccess?.()
          return
        }
        await updateIdentityProfile(currentProfile.id, delta)
        toast.success(t('Identity profile updated'))
      }
      onSuccess?.()
    } catch (error) {
      // 透出真实后端 message（含 Create 唯一约束冲突），绝不吞成 Operation failed。
      handleServerError(error)
    } finally {
      setIsSubmitting(false)
    }
  }

  const rateLimitEnabled = form.watch('rate_limit_enabled')

  return (
    <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
      <SheetHeader className={sideDrawerHeaderClassName()}>
        <SheetTitle>
          {isEdit ? t('Edit Identity Profile') : t('Create Identity Profile')}
        </SheetTitle>
        <SheetDescription>
          {isEdit
            ? t('Edit the current-mode identity attributes. The identity mode, attribution target and assurance are fixed.')
            : t('Bind a NewAPI token to a governed identity profile.')}{' '}
          {t('Click save when you&apos;re done.')}
        </SheetDescription>
      </SheetHeader>

      <Form {...form}>
        <form
          id='identity-profile-form'
          onSubmit={form.handleSubmit(onSubmit)}
          className={sideDrawerFormClassName()}
        >
          <fieldset disabled={isSubmitting} className='contents'>
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='token_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('API Key')}</FormLabel>
                    <FormControl>
                      {isEdit ? (
                        <div className='flex flex-col gap-0.5 rounded-lg border bg-muted/30 px-3 py-2'>
                          <span className='text-sm font-medium'>
                            {currentDetail?.token.token_name}
                          </span>
                          <span className='text-muted-foreground text-xs'>
                            {t('Token ID')}: {currentDetail?.token.token_id}
                          </span>
                        </div>
                      ) : (
                        <TokenSelect
                          value={field.value}
                          onChange={field.onChange}
                          showTokenId
                        />
                      )}
                    </FormControl>
                    <FormDescription>
                      {isEdit
                        ? t('A token is permanently bound to a profile and cannot be changed.')
                        : t('Token ID cannot be changed after creation.')}
                    </FormDescription>
                    {!isEdit &&
                      duplicateState !== 'none' &&
                      duplicateState !== 'free' && (
                        <DuplicateGate state={duplicateState} onRetry={() => void dupQuery.refetch()} />
                      )}
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <FormField
                control={form.control}
                name='identity_mode'
                render={() => (
                  <FormItem>
                    <FormLabel>{t('Identity Configuration')}</FormLabel>
                    {isEdit ? (
                      <div className='flex items-start gap-2 rounded-lg border bg-muted/30 px-3 py-2'>
                        <ShieldCheck className='text-muted-foreground mt-0.5 size-4 shrink-0' />
                        <span className='text-sm'>
                          {identityMode} · {attributionTarget} · {assurance}
                        </span>
                      </div>
                    ) : (
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
                    )}
                    <FormDescription>
                      {t('The identity mode, attribution target and assurance form a legal combination. The assurance level is set by the combination, not chosen freely.')}
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
                          defaultLabel={
                            isEdit
                              ? currentDetail?.principal?.principal_name
                              : undefined
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('The person accountable for this weak-identity key. New candidates must be enabled.')}
                      </FormDescription>
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
                          defaultLabel={
                            isEdit
                              ? currentDetail?.purpose?.credential_purpose_name
                              : undefined
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('What this API key is approved for (WorkBuddy, IDE, script). This does not mean the client is technically verified.')}
                      </FormDescription>
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
                      <FormDescription>
                        {t('Caller is a platform or caller identifier verifiable by the signed identity protocol.')}
                      </FormDescription>
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
                      <FormDescription>
                        {t('A human-friendly label for the caller. Optional.')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SideDrawerSection>
            )}

            {!isEdit && config?.usesApps && (
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
                        />
                      </FormControl>
                      <FormDescription>
                        {config.appMin === config.appMax
                          ? t('Select exactly {{n}} application', {
                              n: config.appMin,
                            })
                          : t('Select at least {{n}} application', {
                              n: config.appMin,
                            })}
                        {' · '}
                        {t('App bindings are set on creation and managed separately.')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SideDrawerSection>
            )}

            <SideDrawerSection>
              <FormField
                control={form.control}
                name='environment'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Environment')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={ENVIRONMENT_DEFAULT} />
                    </FormControl>
                    <FormDescription>
                      {t('A non-empty label of up to {{n}} characters. Defaults to prod.', {
                        n: 32,
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <FormField
                control={form.control}
                name='rate_limit_enabled'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between gap-3'>
                    <div>
                      <FormLabel>{t('Enable Credential Rate Limit')}</FormLabel>
                      <FormDescription>
                        {t('Limit requests for this profile. The backend remains the final boundary.')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        aria-label={t('Enable Credential Rate Limit')}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
              {rateLimitEnabled && (
                <div className='grid gap-4'>
                  <FormField
                    control={form.control}
                    name='rate_limit_window_seconds'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Rate Limit Window (seconds)')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            inputMode='numeric'
                            min={10}
                            max={3600}
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='rate_limit_max_requests'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Max Requests')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            inputMode='numeric'
                            min={1}
                            max={100000}
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          {t('Between {{min}} and {{max}} per window.', {
                            min: 1,
                            max: 100000,
                          })}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}
            </SideDrawerSection>

            {!isEdit && (
              <SideDrawerSection>
                <p className='text-muted-foreground text-xs leading-5'>
                  <AlertCircle className='text-muted-foreground mr-1 inline size-3.5' />
                  {t('An identity profile is created disabled. Enable it explicitly after creation; DYNAMIC and HYBRID modes also require a signing key (configured later).')}
                </p>
              </SideDrawerSection>
            )}
          </fieldset>
        </form>
      </Form>

      <SheetFooter className={sideDrawerFooterClassName()}>
        <SheetClose render={<Button variant='outline' />}>
          {t('Close')}
        </SheetClose>
        <Button
          form='identity-profile-form'
          type='submit'
          disabled={submitBlocked}
        >
          {isSubmitting ? t('Saving...') : t('Save changes')}
        </Button>
      </SheetFooter>
    </SheetContent>
  )
}

/**
 * Token 重复探测四态（§11-C §C.2 一）：
 * idle（未选 Token 不查询）/ querying（正在查询）/ exists（已存在→阻止+提示）/
 * error（查询失败→阻止+Error+Retry）。free（total=0）时不渲染本组件。
 */
function DuplicateGate({
  state,
  onRetry,
}: {
  state: 'idle' | 'querying' | 'error' | 'exists'
  onRetry: () => void
}) {
  const { t } = useTranslation()
  if (state === 'idle') return null
  if (state === 'querying') {
    return (
      <p className='text-muted-foreground text-xs'>{t('Checking...')}</p>
    )
  }
  if (state === 'error') {
    return (
      <div className='flex flex-col gap-1.5'>
        <p className='text-destructive text-xs'>
          {t('Failed to check whether this token is already bound')}
        </p>
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='w-fit'
          onClick={onRetry}
        >
          {t('Retry')}
        </Button>
      </div>
    )
  }
  return (
    <p className='text-destructive text-xs'>
      {t('This API key already has an identity profile. Open the existing profile instead.')}
    </p>
  )
}
