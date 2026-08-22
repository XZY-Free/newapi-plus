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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { PageFooterProvider } from '@/components/layout/components/page-footer'
import { listIdentityAuditEvents } from '@/features/ai-governance/api'
import type { GovernanceAuditEvent } from '@/features/ai-governance/types'
import { getApiKeys } from '@/features/keys/api'

import { IdentityAuditPage } from '../identity-audit/identity-audit-page'

vi.mock('@/features/ai-governance/api', () => ({
  listIdentityAuditEvents: vi.fn(),
}))

vi.mock('@/features/keys/api', () => ({
  getApiKeys: vi.fn(),
  searchApiKeys: vi.fn(),
}))

const event: GovernanceAuditEvent = {
  id: 41,
  created_at: 1700000000,
  request_id: 'req_identity_7f3a',
  token_id: 7,
  profile_id: 12,
  caller_id: 'svc-pipeline',
  principal_id: 5,
  credential_purpose_id: 4,
  identity_mode: 'HYBRID',
  identity_assurance: 'HYBRID_VERIFIED_CONTEXT',
  result: 'REJECTED',
  reason_code: 'SIGNATURE_INVALID',
  claimed_root_app_id: 'app-crm-ingest',
  http_method: 'POST',
  request_path: '/v1/chat/completions',
  client_ip: '203.0.113.9',
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

describe('Identity Audit page', () => {
  test('loads and renders the audit event list', async () => {
    vi.mocked(listIdentityAuditEvents).mockResolvedValue({
      items: [event],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityAuditPage />)

    expect(
      await screen.findByText('req_identity_7f3a')
    ).toBeInTheDocument()
    expect(screen.getByText('REJECTED')).toBeInTheDocument()
    expect(screen.getByText('SIGNATURE_INVALID')).toBeInTheDocument()
    expect(screen.getByText('HYBRID')).toBeInTheDocument()
    expect(screen.getByText('HYBRID_VERIFIED_CONTEXT')).toBeInTheDocument()
    expect(screen.getByText('app-crm-ingest')).toBeInTheDocument()
    expect(listIdentityAuditEvents).toHaveBeenCalled()
  })

  test('renders the empty state when no events match', async () => {
    vi.mocked(listIdentityAuditEvents).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityAuditPage />)

    expect(
      await screen.findByText('No identity audit events found')
    ).toBeInTheDocument()
  })

  test('enters the error state on failure rather than showing stale data', async () => {
    vi.mocked(listIdentityAuditEvents).mockRejectedValue(new Error('boom'))
    renderPage(<IdentityAuditPage />)

    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
    expect(screen.queryByText('req_identity_7f3a')).not.toBeInTheDocument()
  })

  test('global search is forwarded as the request_id backend param', async () => {
    vi.mocked(listIdentityAuditEvents).mockResolvedValue({
      items: [event],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityAuditPage />)
    await screen.findByText('req_identity_7f3a')

    const search = screen.getByPlaceholderText('Search by request ID...')
    await userEvent.type(search, 'req_identity_7f3a')

    await waitFor(() =>
      expect(listIdentityAuditEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ request_id: 'req_identity_7f3a' })
      )
    )
  })

  test('token selector filters by token_id through the real backend param', async () => {
    vi.mocked(listIdentityAuditEvents).mockResolvedValue({
      items: [event],
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
            name: 'Pipeline Key',
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
    renderPage(<IdentityAuditPage />)
    await screen.findByText('req_identity_7f3a')

    await userEvent.click(screen.getByRole('combobox', { name: 'Select API key' }))
    const option = await screen.findByText('Pipeline Key')
    await userEvent.click(option)

    await waitFor(() =>
      expect(listIdentityAuditEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ token_id: 7 })
      )
    )
  })

  test('profile ID input filters by profile_id through the real backend param', async () => {
    vi.mocked(listIdentityAuditEvents).mockResolvedValue({
      items: [event],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityAuditPage />)
    await screen.findByText('req_identity_7f3a')

    await userEvent.type(screen.getByLabelText('Profile ID'), '12')

    await waitFor(() =>
      expect(listIdentityAuditEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ profile_id: 12 })
      )
    )
  })

  test('detail drawer separates profile snapshot from verification outcome and never fabricates client_verified', async () => {
    vi.mocked(listIdentityAuditEvents).mockResolvedValue({
      items: [event],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<IdentityAuditPage />)
    await screen.findByText('req_identity_7f3a')

    await userEvent.click(screen.getByRole('button', { name: 'View Details' }))

    // 三个分组：请求上下文 / Profile 快照（配置事实）/ 验证结果（运行时事实）
    expect(await screen.findByText('Request Context')).toBeInTheDocument()
    expect(screen.getByText('Profile Snapshot')).toBeInTheDocument()
    expect(screen.getByText('Verification Outcome')).toBeInTheDocument()

    // 请求上下文事实
    expect(screen.getByText('POST')).toBeInTheDocument()
    expect(screen.getByText('203.0.113.9')).toBeInTheDocument()
    expect(screen.getByText('/v1/chat/completions')).toBeInTheDocument()

    // 结果/原因/声称 root_app（表格行与抽屉同时存在，取全部命中即可）
    expect(screen.getAllByText('SIGNATURE_INVALID').length).toBeGreaterThan(0)

    // 明示后端无 client_verified，绝不伪造
    expect(
      screen.getByText(
        /The backend audit event has no client_verified field/
      )
    ).toBeInTheDocument()
    expect(
      screen.queryByText(/client was verified/i)
    ).not.toBeInTheDocument()
  })
})
