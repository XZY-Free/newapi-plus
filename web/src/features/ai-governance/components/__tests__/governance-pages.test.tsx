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
  updateBusinessDomain,
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
