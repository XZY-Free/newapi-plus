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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  generateSigningKey,
  getIdentityProfile,
  listApplications,
  listSigningKeys,
  replaceIdentityProfileAppBindings,
  revokeSigningKey,
  rotateSigningKey,
  updateIdentityProfile,
} from '@/features/ai-governance/api'
import type {
  GovernanceIdentityProfileDetail,
  GovernanceSigningKey,
} from '@/features/ai-governance/types'

import { IdentityProfileDetailSheet } from '../identity-profiles/identity-profile-detail'
import { ReconfigureIdentityModeSheet } from '../identity-profiles/reconfigure-identity-mode'

vi.mock('@/features/ai-governance/api', () => ({
  getIdentityProfile: vi.fn(),
  listSigningKeys: vi.fn(),
  generateSigningKey: vi.fn(),
  rotateSigningKey: vi.fn(),
  revokeSigningKey: vi.fn(),
  replaceIdentityProfileAppBindings: vi.fn(),
  updateIdentityProfile: vi.fn(),
  listApplications: vi.fn(),
  listPrincipals: vi.fn(),
  listCredentialPurposes: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const mockedToast = vi.mocked(toast, true)

function baseDetail(
  overrides: Partial<GovernanceIdentityProfileDetail['profile']> = {}
): GovernanceIdentityProfileDetail {
  return {
    profile: {
      id: 1,
      token_id: 7,
      identity_mode: 'DYNAMIC',
      attribution_target_type: 'PLATFORM',
      identity_assurance: 'SIGNED_CONTEXT',
      caller_id: 'platform-a',
      caller_name: 'Platform A',
      principal_id: 0,
      credential_purpose_id: 0,
      environment: 'prod',
      rate_limit_enabled: false,
      rate_limit_window_seconds: 0,
      rate_limit_max_requests: 0,
      enabled: false,
      created_at: 1,
      updated_at: 2,
      ...overrides,
    },
    principal: null,
    purpose: null,
    token: {
      token_id: 7,
      token_name: 'My WorkBuddy Key',
      status: 1,
      expired_time: -1,
      unlimited: false,
      ip_restricted: false,
      model_restricted: false,
      remain_quota: 100,
      created_time: 1,
    },
    bindings: [
      {
        id: 1,
        app_id: 11,
        enabled: true,
        app_code: 'hr-bot',
        app_name: 'HR Bot',
        business_domain_name: 'HR',
        owner_team_name: 'Core',
      },
    ],
    risk: {
      ip_restricted: false,
      model_restricted: false,
      quota_restricted: true,
      expiry_configured: false,
      rate_limit_enabled: false,
      credential_only: true,
      rotation_overdue: true,
      rotation_overdue_days: 3,
      risk_level: 'MEDIUM_RISK',
    },
    rate_limits: {
      items: [],
      config: { enabled: false, window_seconds: 0, max_requests: 0 },
    },
  }
}

function activeKey(status: GovernanceSigningKey['status']): GovernanceSigningKey {
  return {
    id: 1,
    profile_id: 1,
    key_id: 'key_abc',
    status,
    not_before: 100,
    expires_at: status === 'RETIRING' ? 86400 + 100 : 200,
    revoked_at: status === 'REVOKED' ? 300 : 0,
    created_at: 100,
    updated_at: 100,
  }
}

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderWithClient(ui: ReactNode, client: QueryClient = makeClient()) {
  render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
  return { client }
}

/** 断言 secret 绝不出现在 QueryCache / MutationCache 的任何缓存数据里。 */
function expectNoSecretInCaches(client: QueryClient, secret: string) {
  const serializedQueries = client
    .getQueryCache()
    .findAll()
    .map((q) => JSON.stringify(q.state.data ?? null))
  expect(
    serializedQueries.every((s) => !s.includes(secret))
  ).toBe(true)
  const serializedMutations = client
    .getMutationCache()
    .getAll()
    .map((m) => JSON.stringify(m.state.data ?? null))
  expect(
    serializedMutations.every((s) => !s.includes(secret))
  ).toBe(true)
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('Identity Profile Detail sheet', () => {
  test('loads via a fresh getIdentityProfile(id) and renders the default tab', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )

    expect(await screen.findByText('My WorkBuddy Key')).toBeInTheDocument()
    expect(getIdentityProfile).toHaveBeenCalledWith(1)
    // 详情绝不把列表行当永远最新的详情：详情打开必走新的 GET。
    expect(screen.getByText('Identity & Assurance')).toBeInTheDocument()
    expect(screen.getByText('platform-a')).toBeInTheDocument()
  })

  test('shows an error state with Retry that refetches on failure', async () => {
    vi.mocked(getIdentityProfile)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(baseDetail())
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('My WorkBuddy Key')).toBeInTheDocument()
    expect(getIdentityProfile).toHaveBeenCalledTimes(2)
  })

  test('Security & Risk tab is read-only and uses the NewAPI rotation copy, with no client-verified claim', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')

    await userEvent.click(screen.getByRole('tab', { name: 'Security & Risk' }))

    // rotation_overdue 文案必须是 NewAPI API Key / Credential，不是 HMAC。
    expect(
      await screen.findByText('NewAPI API Key / Credential Rotation Overdue')
    ).toBeInTheDocument()
    expect(screen.getByText('3 days since last rotation')).toBeInTheDocument()
    // 只读页绝不出现 client_verified 的“已验证客户端”声称。
    expect(screen.queryByText('Client Verified')).not.toBeInTheDocument()
    expect(screen.queryByText('HMAC')).not.toBeInTheDocument()
  })

  test('Reconfigure is blocked while the profile is enabled', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(
      baseDetail({ enabled: true })
    )
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')

    const reconfigure = screen.getByRole('button', {
      name: 'Reconfigure Identity Mode',
    })
    expect(reconfigure).toBeDisabled()
  })

  test('Token Security Status maps Expired / Exhausted / Unknown, not collapsed to Disabled (P1-5)', async () => {
    const cases: Array<[number, string]> = [
      [3, 'Expired'],
      [4, 'Exhausted'],
      [99, 'Unknown'],
    ]
    for (const [status, label] of cases) {
      const detail = baseDetail()
      detail.token = { ...detail.token, status }
      vi.mocked(getIdentityProfile).mockResolvedValue(detail)
      renderWithClient(
        <IdentityProfileDetailSheet
          profileId={1}
          open
          onOpenChange={() => undefined}
          onChanged={() => undefined}
        />
      )
      await screen.findByText('My WorkBuddy Key')
      await userEvent.click(screen.getByRole('tab', { name: 'Security & Risk' }))
      expect(await screen.findByText(label)).toBeInTheDocument()
      cleanup()
    }
  })
})

describe('Signing Keys lifecycle (DYNAMIC/HYBRID only)', () => {
  test('STATIC profiles never offer key generation', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(
      baseDetail({ identity_mode: 'STATIC', attribution_target_type: 'PRINCIPAL', identity_assurance: 'CREDENTIAL_ONLY' })
    )
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')

    // STATIC 无 Signing Keys Tab，也绝不出现 Generate。
    expect(screen.queryByRole('tab', { name: 'Signing Keys' })).not.toBeInTheDocument()
    expect(screen.queryByText('Generate Signing Key')).not.toBeInTheDocument()
  })

  test('DYNAMIC without an ACTIVE key offers Generate, shows the secret once, and never re-shows it', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    vi.mocked(listSigningKeys).mockResolvedValue([])
    vi.mocked(generateSigningKey).mockResolvedValue({
      key: activeKey('ACTIVE'),
      secret: 'sk_live_s3cret',
    })
    const { client } = renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')
    await userEvent.click(screen.getByRole('tab', { name: 'Signing Keys' }))

    const generateCalls = vi.mocked(listSigningKeys).mock.calls.length
    await userEvent.click(
      await screen.findByRole('button', { name: 'Generate Signing Key' })
    )

    // One-time Secret 弹窗：仅显示一次。
    expect(generateSigningKey).toHaveBeenCalledWith(1)
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Signing Key Secret',
    })
    expect(within(dialog).getByText('sk_live_s3cret')).toBeInTheDocument()

    // P1-2：Generate 后必须重拉真实 listSigningKeys，让 UI 进入真实 ACTIVE 状态。
    await waitFor(() =>
      expect(vi.mocked(listSigningKeys).mock.calls.length).toBeGreaterThan(generateCalls)
    )

    // 关闭即清除；重新打开（未再次 Generate）不得恢复 secret。
    await userEvent.click(within(dialog).getByRole('button', { name: 'Done' }))
    await waitFor(() =>
      expect(screen.queryByText('sk_live_s3cret')).not.toBeInTheDocument()
    )
    // P1-1：secret 只出现在组件临时 state，QueryCache / MutationCache 一律读不到。
    expectNoSecretInCaches(client, 'sk_live_s3cret')
    // 列表仅显示 key 元数据，绝不含 secret。
    expect(screen.queryByText('sk_live_s3cret')).not.toBeInTheDocument()
  })

  test('DYNAMIC with an ACTIVE key offers Rotate (not Generate), rotates, refetches and never caches the secret', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    vi.mocked(listSigningKeys).mockResolvedValue([activeKey('ACTIVE')])
    vi.mocked(rotateSigningKey).mockResolvedValue({
      key: { ...activeKey('ACTIVE'), key_id: 'key_new' },
      secret: 'sk_rotated_secret',
    })
    const { client } = renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')
    await userEvent.click(screen.getByRole('tab', { name: 'Signing Keys' }))

    // 有 ACTIVE 时显示 Rotate，绝不显示 "Regenerate" / Generate。
    expect(
      await screen.findByRole('button', { name: 'Rotate Signing Key' })
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Generate Signing Key' })).not.toBeInTheDocument()
    expect(screen.queryByText('Regenerate')).not.toBeInTheDocument()

    const rotateCalls = vi.mocked(listSigningKeys).mock.calls.length
    await userEvent.click(
      screen.getByRole('button', { name: 'Rotate Signing Key' })
    )
    expect(rotateSigningKey).toHaveBeenCalledWith(1)
    // 24 小时宽限期文案。
    expect(
      await screen.findByText(
        'Rotating retires the current ACTIVE signing key into a 24-hour grace period and issues a new ACTIVE key. The old signing key keeps a 24-hour grace period.'
      )
    ).toBeInTheDocument()
    // P1-2：Rotate 后必须重拉真实 listSigningKeys（旧 ACTIVE→RETIRING、新→ACTIVE）。
    await waitFor(() =>
      expect(vi.mocked(listSigningKeys).mock.calls.length).toBeGreaterThan(rotateCalls)
    )
    // P1-1：rotated secret 同样绝不出现在任何缓存里。
    expectNoSecretInCaches(client, 'sk_rotated_secret')
  })

  test('revoke requires confirmation, revokes, and refetches the signing key list', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    vi.mocked(listSigningKeys).mockResolvedValue([activeKey('ACTIVE')])
    vi.mocked(revokeSigningKey).mockResolvedValue({ revoked: true })
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')
    await userEvent.click(screen.getByRole('tab', { name: 'Signing Keys' }))
    await screen.findByText('key_abc')

    const revokeCalls = vi.mocked(listSigningKeys).mock.calls.length
    await userEvent.click(screen.getAllByRole('button', { name: 'Revoke' })[0])
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Revoke this signing key?',
    })
    await userEvent.click(within(confirm).getByRole('button', { name: 'Revoke' }))

    await waitFor(() =>
      expect(revokeSigningKey).toHaveBeenCalledWith(1, 'key_abc')
    )
    expect(mockedToast.success).toHaveBeenCalledWith('Signing key revoked')
    // P1-2：Revoke 后重拉真实 listSigningKeys（对应 Key 显示 REVOKED）。
    await waitFor(() =>
      expect(vi.mocked(listSigningKeys).mock.calls.length).toBeGreaterThan(revokeCalls)
    )
  })
})

describe('App Bindings whole-set replace', () => {
  test('save sends a whole-set replace body { app_ids }', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    vi.mocked(listSigningKeys).mockResolvedValue([])
    vi.mocked(listApplications).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
    })
    vi.mocked(replaceIdentityProfileAppBindings).mockResolvedValue([])
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')
    await userEvent.click(screen.getByRole('tab', { name: 'App Bindings' }))

    // 历史已停用的已绑定应用仍以真实名称/code + Disabled 可见（经 selectedMeta）。
    // 选择器 chip 与 Current Bindings 列表都渲染真实名称。
    const hrbots = await screen.findAllByText('HR Bot')
    expect(hrbots.length).toBeGreaterThan(0)
    expect(screen.getAllByText(/(hr-bot)/).length).toBeGreaterThan(0)

    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(replaceIdentityProfileAppBindings).toHaveBeenCalledWith(1, {
        app_ids: [11],
      })
    )
    expect(mockedToast.success).toHaveBeenCalledWith('App bindings updated')
  })

  test('a disabled application shows Disabled even when the binding row itself is enabled (P1-4)', async () => {
    // binding.enabled=true，但 Application 不在 enabled 候选集（即当前已停用）。
    // 状态必须来自 Application 当前事实，而不是 binding.enabled。
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    vi.mocked(listSigningKeys).mockResolvedValue([])
    mockEmptyApps() // 应用 11 不在 enabled 候选集 → application.enabled=false
    vi.mocked(replaceIdentityProfileAppBindings).mockResolvedValue([])
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')
    await userEvent.click(screen.getByRole('tab', { name: 'App Bindings' }))

    // 应用 11 仍以真实名称/code 显示，且明确标记 Disabled（Application 状态）。
    await screen.findAllByText('HR Bot')
    const disabledBadges = await screen.findAllByText('Disabled')
    expect(disabledBadges.length).toBeGreaterThan(0)
  })
})

/** Reconfigure 表单渲染 ApplicationMultiSelect + useSelectedApplicationMeta，
 * 两者都会调用 listApplications（P1-4 读 Application 当前 enabled）。 */
function mockEmptyApps() {
  vi.mocked(listApplications).mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 50,
  })
}

describe('Reconfigure Identity Mode flow', () => {
  test('applies App Bindings first, then the profile patch', async () => {
    vi.mocked(replaceIdentityProfileAppBindings).mockResolvedValue([])
    vi.mocked(updateIdentityProfile).mockResolvedValue(baseDetail().profile)
    mockEmptyApps()
    const onFailed = vi.fn()
    renderWithClient(
      <ReconfigureIdentityModeSheet
        detail={baseDetail()}
        open
        onOpenChange={() => undefined}
        onSuccess={() => undefined}
        onFailed={onFailed}
      />
    )

    const callerInput = screen.getByLabelText('Caller ID')
    await userEvent.clear(callerInput)
    await userEvent.type(callerInput, 'platform-b')
    // 默认值带出旧 caller_name，重配置后应显式清空隐藏/旧的名称字段。
    await userEvent.clear(screen.getByLabelText('Caller Name'))
    await userEvent.click(screen.getByRole('button', { name: 'Reconfigure' }))

    await waitFor(() =>
      expect(replaceIdentityProfileAppBindings).toHaveBeenCalledWith(1, {
        app_ids: [11],
      })
    )
    await waitFor(() =>
      expect(updateIdentityProfile).toHaveBeenCalledWith(1, {
        identity_mode: 'DYNAMIC',
        attribution_target_type: 'PLATFORM',
        identity_assurance: 'SIGNED_CONTEXT',
        caller_id: 'platform-b',
        caller_name: '',
        principal_id: 0,
        credential_purpose_id: 0,
      })
    )
    // 成功路径不触发失败强制重拉。
    expect(onFailed).not.toHaveBeenCalled()
  })

  test('a failed profile patch surfaces the error, does not auto-enable, and forces a detail refetch (P1-3)', async () => {
    vi.mocked(replaceIdentityProfileAppBindings).mockResolvedValue([])
    vi.mocked(updateIdentityProfile).mockRejectedValue(new Error('patch boom'))
    mockEmptyApps()
    const onFailed = vi.fn()
    renderWithClient(
      <ReconfigureIdentityModeSheet
        detail={baseDetail()}
        open
        onOpenChange={() => undefined}
        onSuccess={() => undefined}
        onFailed={onFailed}
      />
    )

    await userEvent.type(screen.getByLabelText('Caller ID'), 'x')
    await userEvent.click(screen.getByRole('button', { name: 'Reconfigure' }))

    // 错误透出（真实后端 message），绝无自动启用成功提示。
    await waitFor(() => expect(mockedToast.error).toHaveBeenCalled())
    expect(mockedToast.success).not.toHaveBeenCalledWith(
      'Identity mode reconfigured'
    )
    // P1-3：任一步失败都必须强制重新拉取真实详情。
    await waitFor(() => expect(onFailed).toHaveBeenCalled())
  })

  test('Replace succeeds but Update fails → the detail sheet forces a real getIdentityProfile refetch (P1-3)', async () => {
    vi.mocked(getIdentityProfile).mockResolvedValue(baseDetail())
    vi.mocked(listSigningKeys).mockResolvedValue([])
    mockEmptyApps()
    vi.mocked(replaceIdentityProfileAppBindings).mockResolvedValue([])
    vi.mocked(updateIdentityProfile).mockRejectedValue(new Error('patch boom'))
    renderWithClient(
      <IdentityProfileDetailSheet
        profileId={1}
        open
        onOpenChange={() => undefined}
        onChanged={() => undefined}
      />
    )
    await screen.findByText('My WorkBuddy Key')
    const profileFetches = vi.mocked(getIdentityProfile).mock.calls.length

    await userEvent.click(
      screen.getByRole('button', { name: 'Reconfigure Identity Mode' })
    )
    const callerInput = await screen.findByLabelText('Caller ID')
    await userEvent.clear(callerInput)
    await userEvent.type(callerInput, 'x')
    await userEvent.click(screen.getByRole('button', { name: 'Reconfigure' }))

    // Bindings 已成功替换、Update 失败：必须重新拉取真实详情（服务器已改变）。
    await waitFor(() => expect(updateIdentityProfile).toHaveBeenCalled())
    await waitFor(() =>
      expect(vi.mocked(getIdentityProfile).mock.calls.length).toBeGreaterThan(profileFetches)
    )
  })
})
