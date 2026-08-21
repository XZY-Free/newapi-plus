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

import { PageFooterProvider } from '@/components/layout/components/page-footer'
import {
  listIdentityProfiles,
  updateIdentityProfile,
} from '@/features/ai-governance/api'
import type { GovernanceIdentityProfileDetail } from '@/features/ai-governance/types'
import { getApiKeys } from '@/features/keys/api'

import { IdentityProfilesPage } from '../identity-profiles/identity-profiles-page'

vi.mock('@/features/ai-governance/api', () => ({
  listIdentityProfiles: vi.fn(),
  updateIdentityProfile: vi.fn(),
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
  principal: {
    principal_id: 5,
    principal_code: 'alice',
    principal_name: 'Alice Chen',
  },
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
  rate_limits: {
    items: [],
    config: { enabled: false, window_seconds: 0, max_requests: 0 },
  },
}

function makeClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
}

function renderPage(ui: ReactNode) {
  const footer = document.createElement('div')
  document.body.appendChild(footer)
  return render(
    <PageFooterProvider container={footer}>
      <QueryClientProvider client={makeClient()}>{ui}</QueryClientProvider>
    </PageFooterProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('Identity Profiles page', () => {
  test('loads and renders the aggregated profile list', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [detail],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityProfilesPage />)

    expect(await screen.findByText('My WorkBuddy Key')).toBeInTheDocument()
    expect(screen.getByText('STATIC')).toBeInTheDocument()
    expect(screen.getByText('CREDENTIAL_ONLY')).toBeInTheDocument()
    expect(screen.getByText('Alice Chen')).toBeInTheDocument()
    expect(screen.getByText('MEDIUM_RISK')).toBeInTheDocument()
    expect(listIdentityProfiles).toHaveBeenCalled()
  })

  test('renders the empty state when no profiles match', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityProfilesPage />)

    expect(
      await screen.findByText('No identity profiles found')
    ).toBeInTheDocument()
  })

  test('enters the error state on failure rather than showing stale data', async () => {
    vi.mocked(listIdentityProfiles).mockRejectedValue(new Error('boom'))
    renderPage(<IdentityProfilesPage />)

    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
    expect(screen.queryByText('My WorkBuddy Key')).not.toBeInTheDocument()
  })

  test('keyword search is forwarded as the caller-search keyword (not token name)', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [detail],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityProfilesPage />)
    await screen.findByText('My WorkBuddy Key')

    const search = screen.getByPlaceholderText(
      'Search by caller ID or caller name...'
    )
    await userEvent.type(search, 'bob')

    await waitFor(() =>
      expect(listIdentityProfiles).toHaveBeenLastCalledWith(
        expect.objectContaining({ keyword: 'bob' })
      )
    )
  })

  test('token selector filters by token_id through the real backend param', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [detail],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(getApiKeys).mockResolvedValue({
      success: true,
      data: {
        items: [
          {
            id: 7,
            name: 'Secret Key A',
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
          },
        ],
        total: 1,
        page: 1,
        page_size: 50,
      },
    })
    renderPage(<IdentityProfilesPage />)
    await screen.findByText('My WorkBuddy Key')

    await userEvent.click(screen.getByRole('combobox', { name: 'Select API key' }))
    const option = await screen.findByText('Secret Key A')
    await userEvent.click(option)

    await waitFor(() =>
      expect(listIdentityProfiles).toHaveBeenLastCalledWith(
        expect.objectContaining({ token_id: 7 })
      )
    )
  })

  test('disables an enabled profile via Update.enabled and refreshes', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [detail],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(updateIdentityProfile).mockResolvedValue({
      ...detail.profile,
      enabled: false,
    })
    renderPage(<IdentityProfilesPage />)
    await screen.findByText('My WorkBuddy Key')

    await userEvent.click(screen.getByRole('button', { name: 'Disable' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Disable this identity profile?',
    })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))

    await waitFor(() =>
      expect(updateIdentityProfile).toHaveBeenCalledWith(1, { enabled: false })
    )
    expect(mockedToast.success).toHaveBeenCalledWith('Identity profile disabled')
  })
})
