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
import { useQuery } from '@tanstack/react-query'
import { AlertCircle, LoaderCircle, RefreshCw, Settings2, ShieldCheck } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
  sideDrawerSectionClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getIdentityProfile } from '../../api'
import { findIdentityProfileModeConfig } from '../../lib/identity-profile-mode'
import type { GovernanceIdentityProfileDetail } from '../../types'
import { EnabledBadge } from '../enabled-badge'
import { AppBindingsTab } from './app-bindings-tab'
import { DetailField } from './detail-field'
import { ReconfigureIdentityModeSheet } from './reconfigure-identity-mode'
import { SecurityRiskTab } from './security-risk-tab'
import { SigningKeysTab } from './signing-keys-tab'

function modeNeedsSigningKey(detail: GovernanceIdentityProfileDetail): boolean {
  return (
    detail.profile.identity_mode === 'DYNAMIC' ||
    detail.profile.identity_mode === 'HYBRID'
  )
}

export function IdentityProfileDetailSheet({
  profileId,
  open,
  onOpenChange,
  onChanged,
}: {
  profileId: number | null
  open: boolean
  onOpenChange: (open: boolean) => void
  /** App Binding / Signing Key / Reconfigure 修改后刷新列表并让详情重新拉取。 */
  onChanged: () => void
}) {
  const { t } = useTranslation()
  const [tab, setTab] = useState('identity')
  const [reconfigureOpen, setReconfigureOpen] = useState(false)

  // 详情必须重新调用 getIdentityProfile(id)，绝不得把列表行当作永远最新的详情。
  const query = useQuery({
    queryKey: ['ai-governance', 'identity-profile', profileId],
    queryFn: () => getIdentityProfile(profileId as number),
    enabled: open && profileId != null,
  })

  useEffect(() => {
    if (open) setTab('identity')
  }, [open])

  const refreshDetail = () => {
    void query.refetch()
    onChanged()
  }

  const closeReconfigure = (didChange: boolean) => {
    setReconfigureOpen(false)
    if (didChange) refreshDetail()
  }

  const detail = query.data
  const needsSigningKey = detail ? modeNeedsSigningKey(detail) : false

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[720px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Identity Profile Details')}</SheetTitle>
          <SheetDescription>
            {t('Read-only identity facts, app bindings, security posture and signing keys for this profile.')}
          </SheetDescription>
        </SheetHeader>

        {query.isLoading && (
          <div className='flex flex-1 flex-col items-center justify-center gap-3 py-12 text-sm text-muted-foreground'>
            <LoaderCircle className='size-5 animate-spin' />
            {t('Loading...')}
          </div>
        )}

        {query.isError && (
          <div className='flex flex-1 flex-col items-center justify-center gap-3 py-12 text-center'>
            <AlertCircle className='text-destructive size-5' />
            <span className='text-sm'>{t('Failed to load')}</span>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void query.refetch()}
            >
              <RefreshCw className='size-4' />
              {t('Retry')}
            </Button>
          </div>
        )}

        {detail && (
          <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
            <Tabs value={tab} onValueChange={setTab} className='min-h-0 flex-1'>
              <TabsList className='border-border/70 m-0 flex w-full justify-start rounded-none border-b bg-transparent'>
                <TabsTrigger value='identity'>
                  {t('Identity & Assurance')}
                </TabsTrigger>
                <TabsTrigger value='bindings'>{t('App Bindings')}</TabsTrigger>
                <TabsTrigger value='security'>{t('Security & Risk')}</TabsTrigger>
                {needsSigningKey && (
                  <TabsTrigger value='signing'>{t('Signing Keys')}</TabsTrigger>
                )}
              </TabsList>

              <TabsContent
                value='identity'
                className='flex-1 overflow-y-auto px-4 py-4 sm:px-6'
              >
                <IdentityAssuranceTab
                  detail={detail}
                  onReconfigure={() => setReconfigureOpen(true)}
                />
              </TabsContent>

              <TabsContent
                value='bindings'
                className='flex-1 overflow-y-auto px-4 py-4 sm:px-6'
              >
                <AppBindingsTab detail={detail} onChanged={refreshDetail} />
              </TabsContent>

              <TabsContent
                value='security'
                className='flex-1 overflow-y-auto px-4 py-4 sm:px-6'
              >
                <SecurityRiskTab detail={detail} />
              </TabsContent>

              {needsSigningKey && (
                <TabsContent
                  value='signing'
                  className='flex-1 overflow-y-auto px-4 py-4 sm:px-6'
                >
                  <SigningKeysTab
                    profile={detail.profile}
                    onChanged={refreshDetail}
                  />
                </TabsContent>
              )}
            </Tabs>

            {reconfigureOpen && (
              <ReconfigureIdentityModeSheet
                detail={detail}
                open={reconfigureOpen}
                onOpenChange={(v) => {
                  if (!v) closeReconfigure(false)
                }}
                onSuccess={() => closeReconfigure(true)}
                onFailed={() => refreshDetail()}
              />
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

function IdentityAssuranceTab({
  detail,
  onReconfigure,
}: {
  detail: GovernanceIdentityProfileDetail
  onReconfigure: () => void
}) {
  const { t } = useTranslation()
  const { profile, principal, purpose, token, risk } = detail
  const config = findIdentityProfileModeConfig(
    profile.identity_mode,
    profile.attribution_target_type,
    profile.identity_assurance
  )
  const disabled = !profile.enabled

  let assuranceNote: ReactNode
  if (profile.identity_assurance === 'CREDENTIAL_ONLY') {
    assuranceNote = (
      <>
        <ShieldCheck className='mt-0.5 size-3.5 shrink-0' />
        {t('The API key credential is attributable; the client itself is not verified.')}
      </>
    )
  } else if (profile.identity_assurance === 'SIGNED_CONTEXT') {
    assuranceNote = <>{t('The caller must present a trusted signed context.')}</>
  } else {
    assuranceNote = <>{t('Fixed application attribution plus signed runtime context.')}</>
  }

  return (
    <div className='flex flex-col gap-6'>
      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Token')}</h3>
        <dl className='grid grid-cols-2 gap-4'>
          <DetailField label={t('API Key')}>
            <span className='font-medium'>{token.token_name}</span>
          </DetailField>
          <DetailField label={t('Token ID')}>{token.token_id}</DetailField>
        </dl>
      </section>

      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Identity Configuration')}</h3>
        <dl className='grid grid-cols-2 gap-4'>
          <DetailField label={t('Identity Mode')}>{profile.identity_mode}</DetailField>
          <DetailField label={t('Attribution Target')}>
            {profile.attribution_target_type}
          </DetailField>
          <DetailField label={t('Identity Assurance')}>
            {profile.identity_assurance}
          </DetailField>
          <DetailField label={t('Environment')}>
            {profile.environment || '—'}
          </DetailField>
          <DetailField label={t('Status')}>
            <EnabledBadge enabled={profile.enabled} />
          </DetailField>
        </dl>

        <p className='mt-3 flex items-start gap-1.5 text-xs leading-5 text-muted-foreground'>
          {assuranceNote}
        </p>
      </section>

      {config?.usesCaller && (
        <section className={sideDrawerSectionClassName()}>
          <h3 className='text-sm font-semibold'>{t('Caller')}</h3>
          <dl className='grid grid-cols-2 gap-4'>
            <DetailField label={t('Caller ID')}>
              {profile.caller_id || '—'}
            </DetailField>
            <DetailField label={t('Caller Name')}>
              {profile.caller_name || '—'}
            </DetailField>
          </dl>
        </section>
      )}

      {(principal || purpose) && (
        <section className={sideDrawerSectionClassName()}>
          <h3 className='text-sm font-semibold'>{t('Attribution')}</h3>
          <dl className='grid grid-cols-2 gap-4'>
            {principal && (
              <DetailField label={t('Principal')}>
                <div className='flex flex-col'>
                  <span className='font-medium'>{principal.principal_name}</span>
                  <span className='text-muted-foreground text-xs'>
                    {principal.principal_code}
                  </span>
                </div>
              </DetailField>
            )}
            {purpose && (
              <DetailField label={t('Credential Purpose')}>
                <div className='flex flex-col'>
                  <span className='font-medium'>
                    {purpose.credential_purpose_name}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {purpose.credential_purpose_code}
                  </span>
                </div>
              </DetailField>
            )}
          </dl>
        </section>
      )}

      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Rate Limit')}</h3>
        <dl className='grid grid-cols-2 gap-4'>
          <DetailField label={t('Enabled')}>
            {profile.rate_limit_enabled ? t('Enabled') : t('Disabled')}
          </DetailField>
          {profile.rate_limit_enabled && (
            <>
              <DetailField label={t('Rate Limit Window (seconds)')}>
                {profile.rate_limit_window_seconds}
              </DetailField>
              <DetailField label={t('Max Requests')}>
                {profile.rate_limit_max_requests}
              </DetailField>
            </>
          )}
        </dl>
      </section>

      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Risk')}</h3>
        <dl className='grid grid-cols-2 gap-4'>
          <DetailField label={t('Risk Level')}>{risk.risk_level}</DetailField>
          <DetailField label={t('Credential Only')}>
            {risk.credential_only ? t('Yes') : t('No')}
          </DetailField>
        </dl>
      </section>

      <section className='flex flex-col gap-2'>
        <Button
          type='button'
          variant='outline'
          onClick={onReconfigure}
          disabled={!disabled}
        >
          <Settings2 className='size-4' />
          {t('Reconfigure Identity Mode')}
        </Button>
        {!disabled && (
          <p className='text-muted-foreground text-xs leading-5'>
            {t('Disable this profile before reconfiguring its identity mode. The identity mode, attribution target and assurance are fixed while the profile is enabled.')}
          </p>
        )}
      </section>
    </div>
  )
}
