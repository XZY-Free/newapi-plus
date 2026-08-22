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
import { Copy, KeyRound, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { sideDrawerSectionClassName } from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { formatTimestamp } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  generateSigningKey,
  listSigningKeys,
  revokeSigningKey,
  rotateSigningKey,
} from '../../api'
import type {
  GovernanceIdentityProfile,
  GovernanceSigningKey,
  GovernanceSigningKeyIssued,
  SigningKeyStatus,
} from '../../types'

function statusVariant(
  status: SigningKeyStatus
): 'success' | 'warning' | 'neutral' {
  if (status === 'ACTIVE') return 'success'
  if (status === 'RETIRING') return 'warning'
  return 'neutral'
}

/**
 * 签名密钥管理页（§11-C §C.4 F–K）。仅 DYNAMIC / HYBRID 模式显示（详情页据此渲染 Tab）。
 *
 * 安全语义（逐条硬约束）：
 * - Generate（无 ACTIVE）与 Rotate（有 ACTIVE）返回的 **secret 只存于本组件临时状态**，
 *   绝不进入 React Query 缓存、全局 store、localStorage / sessionStorage / URL、toast、
 *   console / 日志。列表只显示 key 元数据，绝不显示 secret / secret_ciphertext。
 * - One-time Secret 弹窗：仅显示一次；关闭即置空 secret；卸载后无 secret；
 *   不可重新查看、无假展示。
 * - Generate / Rotate **不得使用 useMutation**：那会把含 secret 的响应存进
 *   MutationCache。改用普通 async + 组件局部 pending/secret 状态，secret 只在
 *   组件 state，QueryCache / MutationCache / 存储一律读不到。
 * - Rotate：旧 ACTIVE → RETIRING → expires_at=now+24h，新 → ACTIVE；文案明确
 *   「旧签名密钥仍有 24 小时宽限期」。无 ACTIVE 时显示 Generate，有 ACTIVE 时显示
 *   Rotate，绝不出现 "Regenerate"。
 * - Revoke：不可逆（ConfirmDialog）；若目标是 ACTIVE 且为当前唯一 ACTIVE，则警告
 *   已启用 DYNAMIC/HYBRID Profile 的签名请求可能立即失败。后端允许紧急撤销，
 *   前端绝不阻止；REVOKED 不可恢复；RETIRING 同样可撤销。
 */
export function SigningKeysTab({
  profile,
  onChanged,
}: {
  profile: GovernanceIdentityProfile
  onChanged: () => void
}) {
  const { t } = useTranslation()

  const keysQuery = useQuery({
    queryKey: ['ai-governance', 'identity-profile', profile.id, 'signing-keys'],
    queryFn: () => listSigningKeys(profile.id),
  })
  const keys = keysQuery.data ?? []
  const hasActive = keys.some((k) => k.status === 'ACTIVE')

  // secret 只存本组件临时状态（One-time Secret 弹窗），绝不下沉到任何缓存/存储。
  const [issuedKey, setIssuedKey] = useState<GovernanceSigningKey | null>(null)
  const [secret, setSecret] = useState<string | null>(null)

  // Generate / Rotate 用普通 async + 局部 pending 态，**绝不使用 useMutation**：
  // TanStack Mutation 会把成功响应存进 MutationCache，而 generate/rotate 的响应含明文
  // secret。改为普通调用后，secret 只出现在组件局部 state，QueryCache / MutationCache
  // 一律读不到。
  const [isIssuing, setIsIssuing] = useState(false)

  // 撤销确认弹窗（不可逆）。
  const [revokeTarget, setRevokeTarget] = useState<GovernanceSigningKey | null>(null)
  const [isRevoking, setIsRevoking] = useState(false)

  const refreshKeys = () => {
    // 生命周期修改后必须重拉真实 listSigningKeys，让 UI 立即进入真实
    // ACTIVE/RETIRING/REVOKED 状态；Detail/List 刷新（onChanged）不替代它。
    void keysQuery.refetch()
    onChanged()
  }

  const handleIssue = async () => {
    setIsIssuing(true)
    try {
      const res: GovernanceSigningKeyIssued = hasActive
        ? await rotateSigningKey(profile.id)
        : await generateSigningKey(profile.id)
      // secret 直接进组件状态，绝不经过 React Query 缓存。
      setIssuedKey(res.key)
      setSecret(res.secret)
      toast.success(
        hasActive ? t('Signing key rotated') : t('Signing key generated')
      )
      refreshKeys()
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsIssuing(false)
    }
  }

  const closeSecretDialog = () => {
    setSecret(null)
    setIssuedKey(null)
  }

  const handleCopySecret = async () => {
    if (secret == null) return
    try {
      await navigator.clipboard.writeText(secret)
      toast.success(t('Secret copied'))
    } catch {
      toast.error(t('Failed to copy'))
    }
  }

  const handleRevoke = async () => {
    if (!revokeTarget) return
    setIsRevoking(true)
    try {
      await revokeSigningKey(profile.id, revokeTarget.key_id)
      toast.success(t('Signing key revoked'))
      setRevokeTarget(null)
      refreshKeys()
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsRevoking(false)
    }
  }

  const revokeIsActive = revokeTarget?.status === 'ACTIVE'
  const onlyActiveCount = keys.filter((k) => k.status === 'ACTIVE').length
  const revokeMayBreak = revokeIsActive && onlyActiveCount === 1

  return (
    <div className='flex flex-col gap-6'>
      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Signing Keys')}</h3>
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => void handleIssue()}
            disabled={isIssuing}
          >
            {hasActive ? (
              <>
                <RefreshCw className='size-4' />
                {t('Rotate Signing Key')}
              </>
            ) : (
              <>
                <KeyRound className='size-4' />
                {t('Generate Signing Key')}
              </>
            )}
          </Button>
        </div>
        {hasActive && (
          <p className='text-muted-foreground text-xs leading-5'>
            {t('Rotating retires the current ACTIVE signing key into a 24-hour grace period and issues a new ACTIVE key. The old signing key keeps a 24-hour grace period.')}
          </p>
        )}
      </section>

      <section className={sideDrawerSectionClassName()}>
        <h3 className='text-sm font-semibold'>{t('Key List')}</h3>
        {keysQuery.isLoading && (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        )}
        {keysQuery.isError && (
          <p className='text-destructive text-sm'>{t('Failed to load')}</p>
        )}
        {!keysQuery.isLoading && !keysQuery.isError && keys.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No signing keys yet. Generate one to begin signing requests.')}
          </p>
        )}
        {keys.length > 0 && (
          <ul className='flex flex-col gap-2'>
            {keys.map((key) => (
              <li
                key={key.key_id}
                className='flex flex-col gap-2 rounded-lg border bg-muted/30 px-3 py-2'
              >
                <div className='flex items-center justify-between gap-3'>
                  <span className='truncate font-mono text-xs'>{key.key_id}</span>
                  <StatusBadge
                    label={key.status}
                    variant={statusVariant(key.status)}
                    copyable={false}
                  />
                </div>
                <dl className='grid grid-cols-2 gap-x-4 gap-y-1 text-xs'>
                  <div className='flex justify-between gap-2'>
                    <dt className='text-muted-foreground'>{t('Not Before')}</dt>
                    <dd className='font-mono'>{formatTimestamp(key.not_before)}</dd>
                  </div>
                  <div className='flex justify-between gap-2'>
                    <dt className='text-muted-foreground'>{t('Expires At')}</dt>
                    <dd className='font-mono'>
                      {key.expires_at > 0 ? formatTimestamp(key.expires_at) : '—'}
                    </dd>
                  </div>
                  {key.status === 'REVOKED' && (
                    <div className='flex justify-between gap-2'>
                      <dt className='text-muted-foreground'>{t('Revoked At')}</dt>
                      <dd className='font-mono'>
                        {key.revoked_at > 0 ? formatTimestamp(key.revoked_at) : '—'}
                      </dd>
                    </div>
                  )}
                  <div className='flex justify-between gap-2'>
                    <dt className='text-muted-foreground'>{t('Created At')}</dt>
                    <dd className='font-mono'>{formatTimestamp(key.created_at)}</dd>
                  </div>
                </dl>
                <div className='flex justify-end'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='text-destructive hover:text-destructive'
                    onClick={() => setRevokeTarget(key)}
                  >
                    <Trash2 className='size-3.5' />
                    {t('Revoke')}
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* One-time Secret 弹窗：仅显示一次。 */}
      <AlertDialog open={secret != null} onOpenChange={(v) => !v && closeSecretDialog()}>
        <AlertDialogContent>
          <AlertDialogHeader className='text-start'>
            <AlertDialogTitle>{t('Signing Key Secret')}</AlertDialogTitle>
            <AlertDialogDescription render={<div className='flex flex-col gap-3' />}>
              <p className='text-sm text-foreground'>
                {t('Shown only once. Save it now — it cannot be viewed again.')}
              </p>
              {issuedKey && (
                <div className='flex flex-col gap-1'>
                  <span className='text-muted-foreground text-xs'>
                    {t('Key ID')}
                  </span>
                  <code className='truncate rounded bg-muted px-2 py-1 font-mono text-xs'>
                    {issuedKey.key_id}
                  </code>
                </div>
              )}
              {secret != null && (
                <div className='flex flex-col gap-1'>
                  <span className='text-muted-foreground text-xs'>{t('Secret')}</span>
                  <div className='flex items-center gap-2'>
                    <code className='min-w-0 flex-1 break-all rounded bg-muted px-2 py-1 font-mono text-xs'>
                      {secret}
                    </code>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => void handleCopySecret()}
                    >
                      <Copy className='size-3.5' />
                      {t('Copy')}
                    </Button>
                  </div>
                </div>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={closeSecretDialog}>
              {t('Done')}
            </AlertDialogCancel>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Revoke（不可逆）确认。 */}
      <ConfirmDialog
        open={revokeTarget != null}
        onOpenChange={(v) => {
          if (!v) setRevokeTarget(null)
        }}
        title={t('Revoke this signing key?')}
        desc={
          revokeTarget == null ? (
            ''
          ) : (
            <span className='flex flex-col gap-2'>
              <span>
                {revokeTarget.status === 'RETIRING'
                  ? t('This is a retiring signing key in its 24-hour grace period. Revoking it is irreversible.')
                  : t('Revoking this signing key is irreversible and it can never be restored.')}
              </span>
              {revokeMayBreak && (
                <span className='text-destructive'>
                  {t('If this is the only ACTIVE signing key, signature requests from enabled DYNAMIC or HYBRID profiles may fail immediately until a new ACTIVE key is generated.')}
                </span>
              )}
            </span>
          )
        }
        confirmText={t('Revoke')}
        destructive
        handleConfirm={() => void handleRevoke()}
        isLoading={isRevoking}
      />
    </div>
  )
}
