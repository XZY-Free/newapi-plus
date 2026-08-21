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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  createIdentityProfile,
  listApplications,
  listCredentialPurposes,
  listIdentityProfiles,
  listPrincipals,
  updateIdentityProfile,
} from '@/features/ai-governance/api'
import { PageFooterProvider } from '@/components/layout/components/page-footer'
import type {
  GovernanceIdentityProfileDetail,
  PagedResult,
} from '@/features/ai-governance/types'
import { getApiKeys } from '@/features/keys/api'
import { Sheet } from '@/components/ui/sheet'

import { IdentityProfileForm } from '../identity-profiles/identity-profile-form'

vi.mock('@/features/ai-governance/api', () => ({
  createIdentityProfile: vi.fn(),
  updateIdentityProfile: vi.fn(),
  listIdentityProfiles: vi.fn(),
  listPrincipals: vi.fn(),
  listCredentialPurposes: vi.fn(),
  listApplications: vi.fn(),
}))

vi.mock('@/features/keys/api', () => ({
  getApiKeys: vi.fn(),
  searchApiKeys: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const mockedToast = vi.mocked(toast, true)

const detail: GovernanceIdentityProfileDetail = {
  profile: {
    id: 1,
    token_id: 7,
    identity_mode: 'STATIC',
    attribution_target_type: 'PRINCIPAL',
    identity_assurance: 'CREDENTIAL_ONLY',
    caller_id: 'alice',
    caller_name: 'Alice',
    principal_id: 5,
    credential_purpose_id: 4,
    environment: 'prod',
    rate_limit_enabled: false,
    rate_limit_window_seconds: 0,
    rate_limit_max_requests: 0,
    enabled: true,
    created_at: 1,
    updated_at: 2,
  },
  principal: { principal_id: 5, principal_code: 'alice', principal_name: 'Alice Chen' },
  purpose: {
    credential_purpose_id: 4,
    credential_purpose_code: 'desktop-client',
    credential_purpose_name: 'Desktop automation',
  },
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
  bindings: [],
  risk: {
    ip_restricted: false,
    model_restricted: false,
    quota_restricted: true,
    expiry_configured: false,
    rate_limit_enabled: false,
    credential_only: true,
    rotation_overdue: false,
    risk_level: 'MEDIUM_RISK',
  },
  rate_limits: { items: [], config: { enabled: false, window_seconds: 0, max_requests: 0 } },
}

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderForm(ui: ReactNode) {
  const footer = document.createElement('div')
  document.body.appendChild(footer)
  return render(
    <PageFooterProvider container={footer}>
      <QueryClientProvider client={makeClient()}>
        {/* IdentityProfileForm 返回 SheetContent，须包在 Sheet 根内。 */}
        <Sheet open>{ui}</Sheet>
      </QueryClientProvider>
    </PageFooterProvider>
  )
}

function tokenOption(id: number, name: string) {
  return {
    id,
    name,
    key: 'sk_abc',
    status: 1,
    remain_quota: 100,
    used_quota: 0,
    unlimited_quota: false,
    expired_time: -1,
    created_time: 1,
    accessed_time: 1,
    group: '',
    auto_groups: null,
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
  }
}

function mockProbe(total: number) {
  vi.mocked(listIdentityProfiles).mockResolvedValue({
    items: [],
    total,
    page: 1,
    page_size: 1,
  })
}

/** 从弹出下拉里点选某一候选项。 */
async function pickOption(triggerLabel: string, optionText: string) {
  await userEvent.click(screen.getByRole('combobox', { name: triggerLabel }))
  const option = await screen.findByText(optionText)
  await userEvent.click(option)
}

async function selectToken(name = 'Secret Key A') {
  await pickOption('Select API key', name)
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(getApiKeys).mockResolvedValue({
    success: true,
    data: { items: [tokenOption(7, 'Secret Key A')], total: 1, page: 1, page_size: 50 },
  })
})

describe('IdentityProfileForm — Token duplicate gate', () => {
  test('disambiguates same-name tokens by Token ID in the candidate list', async () => {
    vi.mocked(getApiKeys).mockResolvedValue({
      success: true,
      data: {
        items: [tokenOption(7, 'Secret Key A'), tokenOption(8, 'Secret Key A')],
        total: 2,
        page: 1,
        page_size: 50,
      },
    })
    mockProbe(0)
    renderForm(<IdentityProfileForm mode='create' />)

    await userEvent.click(
      screen.getByRole('combobox', { name: 'Select API key' })
    )
    expect(await screen.findByText('Token ID: 7')).toBeInTheDocument()
    expect(screen.getByText('Token ID: 8')).toBeInTheDocument()
    expect(screen.getAllByText('Secret Key A')).toHaveLength(2)
  })

  test('blocks create when the token already has a profile (exists)', async () => {
    mockProbe(1)
    renderForm(<IdentityProfileForm mode='create' />)
    await selectToken()

    expect(
      await screen.findByText(
        'This API key already has an identity profile. Open the existing profile instead.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save changes' })
    ).toBeDisabled()
  })

  test('blocks create while the probe is pending, then allows when free', async () => {
    let resolveProbe!: (v: PagedResult<GovernanceIdentityProfileDetail>) => void
    vi.mocked(listIdentityProfiles).mockReturnValue(
      new Promise((res) => {
        resolveProbe = res
      })
    )
    renderForm(<IdentityProfileForm mode='create' />)
    await selectToken()

    expect(screen.getByText('Checking...')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save changes' })
    ).toBeDisabled()

    resolveProbe({ items: [], total: 0, page: 1, page_size: 1 })
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Save changes' })
      ).toBeEnabled()
    )
  })

  test('blocks create and shows Error + Retry when the probe fails', async () => {
    vi.mocked(listIdentityProfiles).mockRejectedValue(new Error('boom'))
    renderForm(<IdentityProfileForm mode='create' />)
    await selectToken()

    expect(
      await screen.findByText('Failed to check whether this token is already bound')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save changes' })
    ).toBeDisabled()
  })
})

describe('IdentityProfileForm — Create', () => {
  beforeEach(() => {
    mockProbe(0)
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [
        {
          id: 5,
          principal_code: 'alice',
          principal_name: 'Alice Chen',
          principal_type: 'PERSON',
          business_domain_id: 1,
          usage_team_id: 1,
          enabled: true,
          created_at: 1,
          updated_at: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 50,
    })
    vi.mocked(listCredentialPurposes).mockResolvedValue({
      items: [
        {
          id: 4,
          purpose_code: 'desktop-client',
          purpose_name: 'Desktop automation',
          purpose_type: 'DESKTOP_CLIENT',
          enabled: true,
          created_at: 1,
          updated_at: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 50,
    })
    vi.mocked(listApplications).mockResolvedValue({
      items: [
        {
          id: 9,
          app_code: 'workbuddy',
          app_name: 'WorkBuddy App',
          business_domain_id: 1,
          owner_team_id: 1,
          enabled: true,
          created_at: 1,
          updated_at: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 50,
    })
    vi.mocked(createIdentityProfile).mockResolvedValue(detail.profile)
  })

  test('STATIC/PRINCIPAL create builds a payload without enabled and with cleared caller/apps', async () => {
    const onSuccess = vi.fn()
    renderForm(<IdentityProfileForm mode='create' onSuccess={onSuccess} />)

    await selectToken()
    await pickOption('Select principal', 'Alice Chen')
    await pickOption('Select credential purpose', 'Desktop automation')

    await userEvent.click(
      screen.getByRole('button', { name: 'Save changes' })
    )

    await waitFor(() =>
      expect(createIdentityProfile).toHaveBeenCalledWith({
        token_id: 7,
        identity_mode: 'STATIC',
        attribution_target_type: 'PRINCIPAL',
        identity_assurance: 'CREDENTIAL_ONLY',
        caller_id: '',
        caller_name: '',
        principal_id: 5,
        credential_purpose_id: 4,
        environment: 'prod',
        rate_limit_enabled: false,
        rate_limit_window_seconds: 0,
        rate_limit_max_requests: 0,
        app_ids: [],
      })
    )
    expect(mockedToast.success).toHaveBeenCalledWith('Identity profile created')
    expect(onSuccess).toHaveBeenCalled()
  })

  test('switching to DYNAMIC/PLATFORM clears the previously-set hidden principal/purpose', async () => {
    renderForm(<IdentityProfileForm mode='create' />)
    await selectToken()

    // 先在 STATIC/PRINCIPAL 填 principal + purpose，制造“残留”的隐藏字段。
    await pickOption('Select principal', 'Alice Chen')
    await pickOption('Select credential purpose', 'Desktop automation')

    // 切到 DYNAMIC/PLATFORM。
    const dynamicLabel = screen
      .getByText('DYNAMIC · PLATFORM · SIGNED_CONTEXT')
      .closest('label')
    expect(dynamicLabel).not.toBeNull()
    await userEvent.click(within(dynamicLabel as HTMLElement).getByRole('radio'))

    await userEvent.type(
      screen.getByPlaceholderText('e.g. platform, bot-svc'),
      'platform-a'
    )
    await pickOption('Select applications', 'WorkBuddy App')

    await userEvent.click(
      screen.getByRole('button', { name: 'Save changes' })
    )

    await waitFor(() =>
      expect(createIdentityProfile).toHaveBeenCalledWith(
        expect.objectContaining({
          identity_mode: 'DYNAMIC',
          attribution_target_type: 'PLATFORM',
          identity_assurance: 'SIGNED_CONTEXT',
          principal_id: 0,
          credential_purpose_id: 0,
          caller_id: 'platform-a',
          app_ids: [9],
        })
      )
    )
  })
})

describe('IdentityProfileForm — Edit', () => {
  test('sends only changed fields; token and core triplet are readonly; no app binding change', async () => {
    renderForm(
      <IdentityProfileForm mode='edit' currentDetail={detail} />
    )

    // Token 只读展示（不是可编辑 combobox），核心三元组以只读文本呈现。
    expect(screen.getByText('My WorkBuddy Key')).toBeInTheDocument()
    expect(
      screen.queryByRole('combobox', { name: 'Select API key' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByText('STATIC · PRINCIPAL · CREDENTIAL_ONLY')
    ).toBeInTheDocument()

    // 修改 environment。
    const envInput = screen.getByPlaceholderText('prod')
    await userEvent.clear(envInput)
    await userEvent.type(envInput, 'staging')

    await userEvent.click(
      screen.getByRole('button', { name: 'Save changes' })
    )

    await waitFor(() =>
      expect(updateIdentityProfile).toHaveBeenCalledWith(1, {
        environment: 'staging',
      })
    )
    // 绝不得调用 replaceIdentityProfileAppBindings（C.2 普通 Edit 不碰 App Bindings）。
    // 这里没有 mock 该函数，只需断言 updateIdentityProfile 的 delta 不含 app_ids 等。
    const delta = vi.mocked(updateIdentityProfile).mock.calls[0][1] as Record<
      string,
      unknown
    >
    expect(delta.app_ids).toBeUndefined()
    expect(delta.token_id).toBeUndefined()
    expect(delta.identity_mode).toBeUndefined()
    expect(delta.attribution_target_type).toBeUndefined()
    expect(delta.identity_assurance).toBeUndefined()
    expect(delta.enabled).toBeUndefined()
  })
})
