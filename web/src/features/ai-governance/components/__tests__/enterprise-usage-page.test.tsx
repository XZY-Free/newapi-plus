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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { PageFooterProvider } from '@/components/layout/components/page-footer'
import {
  listBusinessDomains,
  listCredentialPurposes,
  listEnterpriseUsage,
  listEnterpriseUsageAnomalies,
  listIdentityProfiles,
  listOwnerTeams,
  listPrincipals,
  listUsageTeams,
  rebuildEnterpriseUsage,
} from '@/features/ai-governance/api'
import type {
  EnterpriseUsageRow,
  GovernanceBusinessDomain,
  GovernanceCredentialPurpose,
  GovernanceIdentityProfileDetail,
  GovernanceOwnerTeam,
  GovernancePrincipal,
  GovernanceUsageTeam,
  PagedResult,
} from '@/features/ai-governance/types'

import { EnterpriseUsagePage } from '../enterprise-usage/enterprise-usage-page'

vi.mock('@/features/ai-governance/api', () => ({
  listEnterpriseUsage: vi.fn(),
  listEnterpriseUsageAnomalies: vi.fn(),
  rebuildEnterpriseUsage: vi.fn(),
  listIdentityProfiles: vi.fn(),
  listPrincipals: vi.fn(),
  listCredentialPurposes: vi.fn(),
  listBusinessDomains: vi.fn(),
  listUsageTeams: vi.fn(),
  listOwnerTeams: vi.fn(),
}))

const row: EnterpriseUsageRow = {
  id: 1,
  bucket_time: 1700000000,
  profile_id: 12,
  principal_id: 5,
  credential_purpose_id: 4,
  usage_business_domain_id: 2,
  usage_team_id: 3,
  caller_key: 'svc-pipeline',
  root_app_code: 'app-crm',
  app_id: 9,
  app_business_domain_id: 2,
  owner_team_id: 1,
  identity_assurance: 'HYBRID_VERIFIED_CONTEXT',
  client_verified: true,
  model_name: 'gpt-4o',
  dimension_hash: 'h1',
  request_count: 100,
  success_count: 90,
  error_count: 10,
  input_tokens: 1000,
  output_tokens: 2000,
  total_tokens: 3000,
  quota_net: 12345,
  duration_ms_total: 5000,
  created_at: 1700000001,
  updated_at: 1700000001,
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

const emptyPage = <T,>(): PagedResult<T> => ({
  items: [] as T[],
  total: 0,
  page: 1,
  page_size: 50,
})

// E.2 P1-E：企业用量 stats 走服务端分页，mock 返回统一 PagedResult。
const usagePage = (items: EnterpriseUsageRow[]): PagedResult<EnterpriseUsageRow> => ({
  items,
  total: items.length,
  page: 1,
  page_size: 50,
})

beforeEach(() => {
  vi.clearAllMocks()
  // 主数据选择器挂载即预取，全部置空以免误命中真实后端。
  vi.mocked(listIdentityProfiles).mockResolvedValue(
    emptyPage<GovernanceIdentityProfileDetail>()
  )
  vi.mocked(listPrincipals).mockResolvedValue(emptyPage<GovernancePrincipal>())
  vi.mocked(listCredentialPurposes).mockResolvedValue(
    emptyPage<GovernanceCredentialPurpose>()
  )
  vi.mocked(listBusinessDomains).mockResolvedValue(
    emptyPage<GovernanceBusinessDomain>()
  )
  vi.mocked(listUsageTeams).mockResolvedValue(emptyPage<GovernanceUsageTeam>())
  vi.mocked(listOwnerTeams).mockResolvedValue(emptyPage<GovernanceOwnerTeam>())
})

describe('Enterprise Usage page', () => {
  test('loads and renders usage stats with client_verified and assurance', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([row]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)

    // caller / root_app / model 单元格为唯一文本。行数据会随兄弟查询的合并
    // 短暂重建，用 waitFor 等待行稳定连接后再断言。
    await waitFor(() =>
      expect(screen.getAllByText('svc-pipeline').length).toBeGreaterThan(0)
    )
    expect(screen.getByText('app-crm')).toBeInTheDocument()
    expect(screen.getByText('gpt-4o')).toBeInTheDocument()
    // 身份可信等级同时出现在筛选 option 与行徽标中，断言至少出现一次。
    expect(
      screen.getAllByText('HYBRID_VERIFIED_CONTEXT').length
    ).toBeGreaterThan(0)
    // client_verified=true 渲染为 Verified（不因带 caller/root_app 字符串而改变）。
    expect(screen.getByText('Verified')).toBeInTheDocument()
    expect(listEnterpriseUsage).toHaveBeenCalledTimes(1)
  })

  test('shows Unverified for client_verified=false rows', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(
      usagePage([
        { ...row, client_verified: false, identity_assurance: 'UNVERIFIED' },
      ])
    )
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)

    await waitFor(() =>
      expect(screen.getAllByText('Unverified').length).toBeGreaterThan(0)
    )
    expect(screen.getAllByText('UNVERIFIED').length).toBeGreaterThan(0)
  })

  test('renders the empty state when no stats match the filters', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)

    expect(
      await screen.findByText('No usage statistics found')
    ).toBeInTheDocument()
  })

  test('enters the error state on stats failure', async () => {
    vi.mocked(listEnterpriseUsage).mockRejectedValue(new Error('boom'))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)

    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
  })

  test('sends the caller_key filter to the backend as a real param', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([row]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)
    await screen.findByText('svc-pipeline')

    await userEvent.type(screen.getByLabelText('Caller Key'), 'svc-pipeline')

    await waitFor(() =>
      expect(listEnterpriseUsage).toHaveBeenLastCalledWith(
        expect.objectContaining({ caller_key: 'svc-pipeline' })
      )
    )
  })

  test('stats sends page/page_size to the backend (server pagination)', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([row]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)
    await screen.findByText('svc-pipeline')

    await waitFor(() =>
      expect(listEnterpriseUsage).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 1, page_size: 20 })
      )
    )
  })

  test('renders avg latency from duration_ms_total / request_count', async () => {
    // 5000ms 总时长 / 100 请求 = 50ms。
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([row]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)
    // 先等待行稳定连接，再断言平均时延单元格（避免骨架态竞态）。
    await screen.findByText('svc-pipeline')
    expect(screen.getByText('50 ms')).toBeInTheDocument()
  })

  test('renders usage anomalies', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([
      {
        profile_id: 12,
        principal_id: 5,
        credential_purpose_id: 4,
        bucket_time: 1700003600,
        metric: 'quota',
        current: 990,
        baseline: 120,
        threshold: 3,
        model_name: 'gpt-4o',
        identity_assurance: 'CREDENTIAL_ONLY',
      },
    ])
    renderPage(<EnterpriseUsagePage />)

    await waitFor(() =>
      expect(screen.getAllByText('quota').length).toBeGreaterThan(0)
    )
    expect(screen.getAllByText('CREDENTIAL_ONLY').length).toBeGreaterThan(0)
    expect(listEnterpriseUsageAnomalies).toHaveBeenCalled()
  })

  test('rebuild is disabled until a real Unix range is picked', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    renderPage(<EnterpriseUsagePage />)

    const section = await screen.findByRole('region', {
      name: 'Rebuild usage projection',
    })
    const rebuildButton = within(section).getByRole('button', {
      name: 'Rebuild Projection',
    })
    expect(rebuildButton).toBeDisabled()
  })

  test('rebuild is a confirmed Root action that reports processed_logs', async () => {
    vi.mocked(listEnterpriseUsage).mockResolvedValue(usagePage([]))
    vi.mocked(listEnterpriseUsageAnomalies).mockResolvedValue([])
    vi.mocked(rebuildEnterpriseUsage).mockResolvedValue({ processed_logs: 42 })
    renderPage(<EnterpriseUsagePage />)

    const section = await screen.findByRole('region', {
      name: 'Rebuild usage projection',
    })
    // Root-only 标记可见。
    expect(within(section).getByText('Root only')).toBeInTheDocument()

    // 选一个有效范围：打开重建时间范围选择器，点 7 Days 预设。
    await userEvent.click(
      within(section).getByRole('button', { name: 'Date Range' })
    )
    await userEvent.click(screen.getByRole('button', { name: '7 Days' }))

    const rebuildButton = within(section).getByRole('button', {
      name: 'Rebuild Projection',
    })
    await waitFor(() => expect(rebuildButton).toBeEnabled())

    // 先确认再执行：点击后出现确认对话框。
    await userEvent.click(rebuildButton)
    const dialog = await screen.findByRole('alertdialog')
    expect(
      within(dialog).getByText('Rebuild Usage Projection?')
    ).toBeInTheDocument()

    // 确认后调用后端并展示返回的 processed_logs。
    await userEvent.click(within(dialog).getByRole('button', { name: 'Rebuild' }))
    await waitFor(() => expect(rebuildEnterpriseUsage).toHaveBeenCalledTimes(1))
    expect(
      await screen.findByText('Last rebuild processed 42 logs.')
    ).toBeInTheDocument()
  })
})
