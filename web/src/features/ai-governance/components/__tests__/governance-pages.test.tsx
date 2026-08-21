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
  createApplication,
  createBusinessDomain,
  createCredentialPurpose,
  listApplications,
  listBusinessDomains,
  listCredentialPurposes,
  listOwnerTeams,
  listPrincipals,
  listUsageTeams,
  updateApplication,
  updateBusinessDomain,
  updatePrincipal,
} from '@/features/ai-governance/api'
import type {
  GovernanceApplication,
  GovernanceBusinessDomain,
  GovernanceCredentialPurpose,
  GovernanceOwnerTeam,
  GovernancePrincipal,
  GovernanceUsageTeam,
} from '@/features/ai-governance/types'
import { ApplicationsPage } from '../applications/applications-page'
import { BusinessDomainsPage } from '../business-domains/business-domains-page'
import { CredentialPurposesPage } from '../credential-purposes/credential-purposes-page'
import { MasterDataSelect } from '../master-data-select'
import { PrincipalsPage } from '../principals/principals-page'

vi.mock('@/features/ai-governance/api', () => ({
  listBusinessDomains: vi.fn(),
  createBusinessDomain: vi.fn(),
  updateBusinessDomain: vi.fn(),
  listUsageTeams: vi.fn(),
  createUsageTeam: vi.fn(),
  updateUsageTeam: vi.fn(),
  listOwnerTeams: vi.fn(),
  createOwnerTeam: vi.fn(),
  updateOwnerTeam: vi.fn(),
  listCredentialPurposes: vi.fn(),
  createCredentialPurpose: vi.fn(),
  updateCredentialPurpose: vi.fn(),
  listPrincipals: vi.fn(),
  createPrincipal: vi.fn(),
  updatePrincipal: vi.fn(),
  listApplications: vi.fn(),
  createApplication: vi.fn(),
  updateApplication: vi.fn(),
  getPrincipal: vi.fn(),
  getApplication: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const mockedToast = vi.mocked(toast, true)

const domain: GovernanceBusinessDomain = {
  id: 1,
  domain_code: 'hr',
  domain_name: 'Human Resources',
  enabled: true,
  created_at: 1,
  updated_at: 2,
}
const usageTeam: GovernanceUsageTeam = {
  id: 2,
  team_code: 'hr-eng',
  team_name: 'HR Engineering',
  enabled: true,
  created_at: 1,
  updated_at: 2,
}
const ownerTeam: GovernanceOwnerTeam = {
  id: 3,
  team_code: 'ai-platform',
  team_name: 'AI Platform',
  enabled: true,
  created_at: 1,
  updated_at: 2,
}
const purpose: GovernanceCredentialPurpose = {
  id: 4,
  purpose_code: 'desktop-client',
  purpose_name: 'Desktop automation',
  purpose_type: 'DESKTOP_CLIENT',
  enabled: true,
  created_at: 1,
  updated_at: 2,
}
const principal: GovernancePrincipal = {
  id: 5,
  principal_code: 'alice',
  principal_name: 'Alice Chen',
  principal_type: 'PERSON',
  business_domain_id: 1,
  usage_team_id: 2,
  enabled: true,
  created_at: 1,
  updated_at: 2,
}
const app: GovernanceApplication = {
  id: 6,
  app_code: 'hr-chat',
  app_name: 'HR Chatbot',
  business_domain_id: 1,
  owner_team_id: 3,
  enabled: true,
  created_at: 1,
  updated_at: 2,
}

function makeClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
}

function renderPage(ui: ReactNode) {
  // DataTablePage renders pagination via PageFooterPortal, which needs a
  // PageFooterProvider container in the tree (otherwise pagination is null).
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

describe('Business Domains page', () => {
  test('loads and renders a list of domains', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<BusinessDomainsPage />)

    expect(await screen.findByText('Human Resources')).toBeInTheDocument()
    expect(screen.getByText('hr')).toBeInTheDocument()
    expect(listBusinessDomains).toHaveBeenCalled()
  })

  test('renders the empty state when no domains match', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    renderPage(<BusinessDomainsPage />)

    expect(
      await screen.findByText('No business domains found')
    ).toBeInTheDocument()
  })

  test('surfaces a real error instead of swallowing it', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    vi.mocked(createBusinessDomain).mockRejectedValue(new Error('boom'))
    renderPage(<BusinessDomainsPage />)
    await screen.findByText('No business domains found')

    await userEvent.click(
      screen.getByRole('button', { name: 'Add Business Domain' })
    )
    await userEvent.type(
      screen.getByPlaceholderText('e.g. hr, finance, production'),
      'fin'
    )
    await userEvent.type(
      screen.getByPlaceholderText('Enter a domain name'),
      'Finance'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(mockedToast.error).toHaveBeenCalled())
  })

  test('searches by keyword (debounced) against the API', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    renderPage(<BusinessDomainsPage />)
    await screen.findByText('No business domains found')

    const search = screen.getByPlaceholderText('Search by code or name...')
    await userEvent.type(search, 'fin')

    await waitFor(
      () =>
        expect(listBusinessDomains).toHaveBeenCalledWith(
          expect.objectContaining({ keyword: 'fin' })
        ),
      { timeout: 2000 }
    )
  })

  test('paginates to the next page against the API', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 25,
      page: 1,
      page_size: 20,
    })
    renderPage(<BusinessDomainsPage />)
    await screen.findByText('Human Resources')

    await userEvent.click(
      screen.getByRole('button', { name: 'Go to next page' })
    )

    await waitFor(() =>
      expect(listBusinessDomains).toHaveBeenCalledWith(
        expect.objectContaining({ page: 2 })
      )
    )
  })

  test('creates a domain and refreshes', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    vi.mocked(createBusinessDomain).mockResolvedValue(domain)
    renderPage(<BusinessDomainsPage />)
    await screen.findByText('No business domains found')

    await userEvent.click(
      screen.getByRole('button', { name: 'Add Business Domain' })
    )
    await userEvent.type(
      screen.getByPlaceholderText('e.g. hr, finance, production'),
      'fin'
    )
    await userEvent.type(
      screen.getByPlaceholderText('Enter a domain name'),
      'Finance'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(createBusinessDomain).toHaveBeenCalledWith({
        domain_code: 'fin',
        domain_name: 'Finance',
      })
    )
    expect(mockedToast.success).toHaveBeenCalledWith('Business domain created')
  })

  test('edits a domain with the code prefilled and read-only', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<BusinessDomainsPage />)
    await screen.findByText('Human Resources')

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))

    // code is prefilled and disabled (read-only after creation)
    const codeInput = screen.getByPlaceholderText(
      'e.g. hr, finance, production'
    )
    expect(codeInput).toHaveValue('hr')
    expect(codeInput).toBeDisabled()
    expect(screen.getByPlaceholderText('Enter a domain name')).toHaveValue(
      'Human Resources'
    )
  })

  test('disables a domain after confirmation', async () => {
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(updateBusinessDomain).mockResolvedValue({
      ...domain,
      enabled: false,
    })
    renderPage(<BusinessDomainsPage />)
    await screen.findByText('Human Resources')

    await userEvent.click(screen.getByRole('button', { name: 'Disable' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Disable this item?',
    })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))

    await waitFor(() =>
      expect(updateBusinessDomain).toHaveBeenCalledWith(
        domain.id,
        expect.objectContaining({ enabled: false })
      )
    )
    expect(mockedToast.success).toHaveBeenCalledWith('Business domain disabled')
  })
})

describe('Credential Purposes page', () => {
  test('renders the purpose type enum label', async () => {
    vi.mocked(listCredentialPurposes).mockResolvedValue({
      items: [purpose],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<CredentialPurposesPage />)

    expect(await screen.findByText('Desktop automation')).toBeInTheDocument()
    expect(screen.getByText('Desktop Client')).toBeInTheDocument()
  })

  test('creates a purpose keeping the raw enum value', async () => {
    vi.mocked(listCredentialPurposes).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    vi.mocked(createCredentialPurpose).mockResolvedValue(purpose)
    renderPage(<CredentialPurposesPage />)
    await screen.findByText('No credential purposes found')

    await userEvent.click(
      screen.getByRole('button', { name: 'Add Credential Purpose' })
    )
    await userEvent.type(
      screen.getByPlaceholderText('e.g. desktop-client, ide, script'),
      'svc'
    )
    await userEvent.type(screen.getByPlaceholderText('Enter a purpose name'), 'Svc')
    // choose Service from the enum select
    await userEvent.click(
      screen.getByRole('combobox', { name: 'Purpose Type' })
    )
    await userEvent.click(screen.getByText('Service'))
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(createCredentialPurpose).toHaveBeenCalledWith({
        purpose_code: 'svc',
        purpose_name: 'Svc',
        purpose_type: 'SERVICE',
      })
    )
  })
})

describe('Principals page', () => {
  test('resolves business domain and usage team names in columns', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [principal],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<PrincipalsPage />)

    await waitFor(() =>
      expect(screen.getByText('Alice Chen')).toBeInTheDocument()
    )
    expect(screen.getByText('Human Resources')).toBeInTheDocument()
    expect(screen.getByText('HR Engineering')).toBeInTheDocument()
    expect(screen.getByText('Person')).toBeInTheDocument()
  })

  test('create drawer offers Business Domain and Usage Team selectors', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })
    renderPage(<PrincipalsPage />)
    await screen.findByText('No principals found')

    await userEvent.click(screen.getByRole('button', { name: 'Add Principal' }))

    expect(
      screen.getByRole('combobox', { name: 'Select business domain' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'Select usage team' })
    ).toBeInTheDocument()
  })
})

describe('Applications page', () => {
  test('resolves owner team name and creates an application', async () => {
    vi.mocked(listApplications).mockResolvedValue({
      items: [app],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listOwnerTeams).mockResolvedValue({
      items: [ownerTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(createApplication).mockResolvedValue(app)
    renderPage(<ApplicationsPage />)

    await waitFor(() =>
      expect(screen.getByText('HR Chatbot')).toBeInTheDocument()
    )
    expect(screen.getByText('AI Platform')).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Add AI Application' })
    )
    expect(
      screen.getByRole('combobox', { name: 'Select owner team' })
    ).toBeInTheDocument()
  })
})

describe('Business Domains page — §11-B.1 边界（Error ≠ Empty + retry）', () => {
  test('列表查询失败渲染错误态而非空态，并可点击重试恢复', async () => {
    vi.mocked(listBusinessDomains)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({
        items: [domain],
        total: 1,
        page: 1,
        page_size: 20,
      })
    renderPage(<BusinessDomainsPage />)

    // 错误态，绝不能被当成空列表
    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
    expect(
      screen.queryByText('No business domains found')
    ).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('Human Resources')).toBeInTheDocument()
  })

  test('切换查询参数后新查询失败进入错误态，而非沿用旧数据冒充正常结果', async () => {
    vi.mocked(listBusinessDomains)
      .mockResolvedValueOnce({
        items: [domain],
        total: 1,
        page: 1,
        page_size: 20,
      })
      .mockRejectedValueOnce(new Error('boom'))
    renderPage(<BusinessDomainsPage />)
    // 第一次查询成功，页面已有数据
    await screen.findByText('Human Resources')

    // 改 keyword 触发新查询 → reject
    await userEvent.type(
      screen.getByPlaceholderText('Search by code or name...'),
      'fin'
    )
    await waitFor(
      () => expect(listBusinessDomains).toHaveBeenCalledWith(
        expect.objectContaining({ keyword: 'fin' })
      ),
      { timeout: 2000 }
    )

    // 必须进入错误态，不得把旧数据当作新查询结果继续正常展示
    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
    expect(screen.queryByText('Human Resources')).not.toBeInTheDocument()
    expect(
      screen.queryByText('No business domains found')
    ).not.toBeInTheDocument()
  })
})

describe('Principals page — §11-B.1 引用与关联边界', () => {
  const enabledDomain: GovernanceBusinessDomain = {
    id: 2,
    domain_code: 'hr-portal',
    domain_name: 'HR Portal',
    enabled: true,
    created_at: 1,
    updated_at: 2,
  }
  const disabledDomain: GovernanceBusinessDomain = {
    id: 1,
    domain_code: 'legacy-hr',
    domain_name: 'Legacy HR',
    enabled: false,
    created_at: 1,
    updated_at: 2,
  }
  const principal201: GovernancePrincipal = {
    ...principal,
    business_domain_id: 201,
  }

  test('引用按 total 分页拉全量（201 条跨两页仍解析出名称）', async () => {
    const domains = Array.from({ length: 201 }, (_, i) => ({
      id: i + 1,
      domain_code: `d${i + 1}`,
      domain_name: `Domain ${i + 1}`,
      enabled: true,
      created_at: 1,
      updated_at: 2,
    }))
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [principal201],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockImplementation((params) => {
      const page = params?.page ?? 1
      const start = (page - 1) * 200
      return Promise.resolve({
        items: domains.slice(start, start + 200),
        total: 201,
        page,
        page_size: 200,
      })
    })
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<PrincipalsPage />)

    await waitFor(() =>
      expect(screen.getByText('Alice Chen')).toBeInTheDocument()
    )
    // 第 2 页的 domain id=201 被解析为名称，而非裸数字
    expect(screen.getByText('Domain 201')).toBeInTheDocument()
    expect(screen.queryByText('201')).not.toBeInTheDocument()
    // 证明确实翻到了第二页
    expect(listBusinessDomains).toHaveBeenCalledWith(
      expect.objectContaining({ page: 2 })
    )
  })

  test('引用查询失败时列展示占位符，而非把数字 ID 当名称', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [principal],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockRejectedValue(new Error('ref down'))
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<PrincipalsPage />)

    await waitFor(() =>
      expect(screen.getByText('Alice Chen')).toBeInTheDocument()
    )
    // business_domain 列失败 → 占位符；usage_team 列仍正常解析出名称
    expect(screen.getAllByText('—').length).toBe(1)
    expect(screen.getByText('HR Engineering')).toBeInTheDocument()
  })

  test('已停用引用仍显示名称，但不进入新建候选列表', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [{ ...principal, business_domain_id: 1 }],
      total: 1,
      page: 1,
      page_size: 20,
    })
    // 引用：不过滤 enabled → 停用的 Legacy HR 也能解析名称；
    // 分配候选：仅 enabled=true → 只返回 HR Portal。
    vi.mocked(listBusinessDomains).mockImplementation((params) =>
      Promise.resolve(
        params?.enabled
          ? { items: [enabledDomain], total: 1, page: 1, page_size: 200 }
          : {
              items: [disabledDomain, enabledDomain],
              total: 2,
              page: 1,
              page_size: 200,
            }
      )
    )
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<PrincipalsPage />)

    // 已停用的历史引用（business_domain_id=1 → Legacy HR）在列中仍显示名称
    await waitFor(() =>
      expect(screen.getByText('Legacy HR')).toBeInTheDocument()
    )

    // 新建抽屉的分配候选只请求 enabled=true
    await userEvent.click(screen.getByRole('button', { name: 'Add Principal' }))
    await userEvent.click(
      screen.getByRole('combobox', { name: 'Select business domain' })
    )
    await waitFor(() =>
      expect(listBusinessDomains).toHaveBeenCalledWith(
        expect.objectContaining({ enabled: true })
      )
    )
    // 停用的 Legacy HR 不进入候选列表；启用的 HR Portal 出现
    const listbox = await screen.findByRole('listbox')
    expect(within(listbox).getByText('HR Portal')).toBeInTheDocument()
    expect(
      within(listbox).queryByText('Legacy HR')
    ).not.toBeInTheDocument()
  })

  test('编辑抽屉用引用名称预填选择器，而非数字 ID', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [principal],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<PrincipalsPage />)

    // 先等引用在列中渲染（同时稳定行，避免点击命中被重绘摘除的节点）
    await screen.findByText('Alice Chen')
    await screen.findByText('Human Resources')
    await screen.findByText('HR Engineering')
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    // business_domain_id=1 → Human Resources；usage_team_id=2 → HR Engineering
    expect(
      await screen.findByRole('combobox', { name: 'Human Resources' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'HR Engineering' })
    ).toBeInTheDocument()
  })

  test('启用/停用仅发送 { enabled }，不夹带其它字段', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [principal],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(updatePrincipal).mockResolvedValue({
      ...principal,
      enabled: false,
    })
    renderPage(<PrincipalsPage />)

    await screen.findByText('Alice Chen')
    await screen.findByText('Human Resources')
    await screen.findByText('HR Engineering')
    await userEvent.click(screen.getByRole('button', { name: 'Disable' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Disable this item?',
    })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))

    await waitFor(() =>
      expect(updatePrincipal).toHaveBeenCalledWith(5, { enabled: false })
    )
  })

  test('仅改名称时重传关联 ID，避免触发 requireEnabled 校验', async () => {
    vi.mocked(listPrincipals).mockResolvedValue({
      items: [principal],
      total: 1,
      page: 1,
      page_size: 20,
    })
    // 父引用已停用：若重传 business_domain_id 会误触发后端 requireEnabled 拒绝
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [{ ...domain, enabled: false }],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listUsageTeams).mockResolvedValue({
      items: [usageTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(updatePrincipal).mockResolvedValue({
      ...principal,
      principal_name: 'New Name',
    })
    renderPage(<PrincipalsPage />)

    await screen.findByText('Alice Chen')
    await screen.findByText('Human Resources')
    await screen.findByText('HR Engineering')
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    const nameInput = await screen.findByPlaceholderText(
      'Enter a principal name'
    )
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'New Name')
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(updatePrincipal).toHaveBeenCalledWith(5, {
        principal_name: 'New Name',
      })
    )
    // 绝不重发未变的关联 ID
    expect(updatePrincipal).not.toHaveBeenCalledWith(
      5,
      expect.objectContaining({ business_domain_id: 1 })
    )
    expect(updatePrincipal).not.toHaveBeenCalledWith(
      5,
      expect.objectContaining({ usage_team_id: 2 })
    )
  })
})

describe('Applications page — §11-B.1 引用与启用边界', () => {
  test('编辑抽屉用 owner team 引用名称预填选择器', async () => {
    vi.mocked(listApplications).mockResolvedValue({
      items: [app],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listOwnerTeams).mockResolvedValue({
      items: [ownerTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<ApplicationsPage />)

    await screen.findByText('HR Chatbot')
    await screen.findByText('AI Platform')
    await screen.findByText('Human Resources')
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    // owner_team_id=3 → AI Platform；business_domain_id=1 → Human Resources
    expect(
      await screen.findByRole('combobox', { name: 'AI Platform' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'Human Resources' })
    ).toBeInTheDocument()
  })

  test('启用/停用仅发送 { enabled }，不夹带其它字段', async () => {
    vi.mocked(listApplications).mockResolvedValue({
      items: [app],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listOwnerTeams).mockResolvedValue({
      items: [ownerTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(updateApplication).mockResolvedValue({ ...app, enabled: false })
    renderPage(<ApplicationsPage />)

    await screen.findByText('HR Chatbot')
    await screen.findByText('AI Platform')
    await screen.findByText('Human Resources')
    await userEvent.click(screen.getByRole('button', { name: 'Disable' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Disable this item?',
    })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Disable' }))

    await waitFor(() =>
      expect(updateApplication).toHaveBeenCalledWith(6, { enabled: false })
    )
  })

  test('带关联筛选的页面切换参数后新查询失败进入错误态，而非沿用旧数据', async () => {
    vi.mocked(listApplications)
      .mockResolvedValueOnce({
        items: [app],
        total: 1,
        page: 1,
        page_size: 20,
      })
      .mockRejectedValueOnce(new Error('boom'))
    vi.mocked(listBusinessDomains).mockResolvedValue({
      items: [domain],
      total: 1,
      page: 1,
      page_size: 20,
    })
    vi.mocked(listOwnerTeams).mockResolvedValue({
      items: [ownerTeam],
      total: 1,
      page: 1,
      page_size: 20,
    })
    renderPage(<ApplicationsPage />)
    await screen.findByText('HR Chatbot')

    // 改 keyword 触发新查询 → reject（Applications 含 business_domain/owner_team 关联筛选）
    await userEvent.type(
      screen.getByPlaceholderText('Search by code or name...'),
      'fin'
    )
    await waitFor(
      () => expect(listApplications).toHaveBeenCalledWith(
        expect.objectContaining({ keyword: 'fin' })
      ),
      { timeout: 2000 }
    )

    expect(
      await screen.findByText('Oops! Something went wrong')
    ).toBeInTheDocument()
    expect(screen.queryByText('HR Chatbot')).not.toBeInTheDocument()
    expect(screen.queryByText('No AI applications found')).not.toBeInTheDocument()
  })
})

describe('MasterDataSelect — §11-B.1 收口（候选 Error ≠ Empty）', () => {
  test('候选请求失败显示错误+Retry 而非空态，Retry 真实重取后选项出现', async () => {
    const fetchPage = vi
      .fn()
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({
        items: [domain],
        total: 1,
        page: 1,
        page_size: 50,
      })
    renderPage(
      <MasterDataSelect<GovernanceBusinessDomain>
        value={null}
        onChange={() => {}}
        queryKey={['test-options']}
        fetchPage={fetchPage}
        itemToValue={(item) => item.id}
        itemToLabel={(item) => item.domain_name}
        placeholder='Select business domain'
        emptyText='No business domain found'
      />
    )

    // 打开下拉
    await userEvent.click(
      screen.getByRole('combobox', { name: 'Select business domain' })
    )

    // 初始查询 reject → Error 态：显示 Failed to load options，不显示 Empty 文案
    expect(
      await screen.findByText('Failed to load options')
    ).toBeInTheDocument()
    expect(screen.queryByText('No business domain found')).not.toBeInTheDocument()

    // Retry 真实调用 refetch → API 成功 → 选项出现
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('Human Resources')).toBeInTheDocument()
    expect(fetchPage).toHaveBeenCalledTimes(2)
  })
})
