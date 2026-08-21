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
/**
 * 企业 AI 治理 API 契约级测试（§11-A.2 收口）。
 *
 * 锁定 `api.ts` 与后端 `/api/ai-governance/*` 的冻结契约：
 * - 列表统一返回 PagedResult（items/total/page/page_size）。
 * - Query 参数显式使用后端 snake_case 键，undefined 可选筛选被剔除。
 * - 写请求使用显式 Create/Update Payload，绝不携带 id/created_at/updated_at。
 * - 创建/更新 Profile 只返回裸 `GovernanceIdentityProfile`，不假装返回聚合 Detail。
 * - Profile 详情 principal/purpose 空对象 `{}` 在 API 边界规范化为 `null`（列表与详情同一 helper）。
 * - App Binding 整体替换返回正式 `GovernanceIdentityAppBinding[]`。
 * - 审计事件允许 identity_mode/identity_assurance 为空串（Profile 未解析）。
 * - 企业用量 stats 返回裸数组；anomalies 的 identity_assurance 为强类型。
 * - 治理首页注册恰好 9 个分区。
 * - 业务编码（domain_code/app_code/principal_code/purpose_code 等）均用小写稳定编码。
 */
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  createBusinessDomain,
  createIdentityProfile,
  generateSigningKey,
  getIdentityProfile,
  listApplications,
  listBusinessDomains,
  listCredentialPurposes,
  listEnterpriseUsage,
  listEnterpriseUsageAnomalies,
  listIdentityAuditEvents,
  listIdentityProfiles,
  listOwnerTeams,
  listPrincipals,
  listSigningKeys,
  listUsageTeams,
  pickDefined,
  rebuildEnterpriseUsage,
  replaceIdentityProfileAppBindings,
  revokeSigningKey,
  rotateSigningKey,
  updateIdentityProfile,
} from './api'
import { AI_GOVERNANCE_SECTION_IDS, AI_GOVERNANCE_SECTION_META } from './section-registry'
import type {
  EnterpriseUsageAnomaly,
  GovernanceAuditEvent,
  GovernanceIdentityAppBinding,
  GovernanceIdentityProfile,
  GovernanceIdentityProfileDetail,
  PagedResult,
} from './types'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}))

const mockedApi = vi.mocked(api, true)

function paged<T>(items: T[], total = items.length, page = 1, page_size = 20): PagedResult<T> {
  return { items, total, page, page_size }
}

/** 后端 buildIdentityProfileDetail 返回的聚合详情样例（真实关联值）。 */
const detail: GovernanceIdentityProfileDetail = {
  profile: {
    id: 11,
    token_id: 7,
    identity_mode: 'STATIC',
    attribution_target_type: 'PRINCIPAL',
    identity_assurance: 'CREDENTIAL_ONLY',
    caller_id: 'caller-1',
    caller_name: 'CLI',
    principal_id: 3,
    credential_purpose_id: 4,
    environment: 'prod',
    rate_limit_enabled: true,
    rate_limit_window_seconds: 3600,
    rate_limit_max_requests: 100,
    enabled: true,
    created_at: 1,
    updated_at: 1,
  },
  principal: { principal_id: 3, principal_code: 'p-3', principal_name: 'Alice' },
  purpose: { credential_purpose_id: 4, credential_purpose_code: 'ide', credential_purpose_name: 'IDE' },
  token: {
    token_id: 7,
    token_name: 't7',
    status: 1,
    expired_time: 0,
    unlimited: false,
    ip_restricted: false,
    model_restricted: false,
    remain_quota: 1000,
    created_time: 1,
  },
  bindings: [
    {
      id: 1,
      app_id: 21,
      enabled: true,
      app_code: 'app-21',
      app_name: 'WorkBuddy',
      business_domain_id: 1,
      business_domain_code: 'hr',
      business_domain_name: 'HR',
      owner_team_id: 2,
      owner_team_code: 'hr-o',
      owner_team_name: 'HR-O',
    },
  ],
  risk: {
    ip_restricted: false,
    model_restricted: false,
    quota_restricted: false,
    expiry_configured: false,
    rate_limit_enabled: true,
    credential_only: true,
    rotation_overdue: false,
    risk_level: 'LOWER_RISK',
  },
  rate_limits: {
    items: [],
    config: { enabled: true, window_seconds: 3600, max_requests: 100 },
  },
}

/** 裸 Profile：POST/PUT `/identity-profiles` 的真实返回，非聚合 Detail。 */
const bareProfile: GovernanceIdentityProfile = detail.profile

/** App Binding 行数组：replace 的真实返回。 */
const bindingRows: GovernanceIdentityAppBinding[] = [
  { id: 1, profile_id: 11, app_id: 21, enabled: true, created_at: 1, updated_at: 1 },
  { id: 2, profile_id: 11, app_id: 22, enabled: false, created_at: 1, updated_at: 1 },
]

/** 审计事件样例（已解析 Profile 时的值）。 */
const auditEvent: GovernanceAuditEvent = {
  id: 1,
  created_at: 1,
  request_id: 'req-1',
  token_id: 7,
  profile_id: 9,
  caller_id: 'caller-1',
  principal_id: 3,
  credential_purpose_id: 4,
  identity_mode: 'STATIC',
  identity_assurance: 'CREDENTIAL_ONLY',
  result: 'REJECTED',
  reason_code: 'NO_PRINCIPAL',
  claimed_root_app_id: '',
  http_method: 'POST',
  request_path: '/v1/chat/completions',
  client_ip: '127.0.0.1',
}

describe('pickDefined', () => {
  test('strips undefined keys so optional filters are never sent as empty values', () => {
    expect(pickDefined({ a: 1, b: undefined, c: 'x' })).toEqual({ a: 1, c: 'x' })
    expect(pickDefined({})).toEqual({})
  })
})

describe('master-data list contract (PagedResult + snake_case query params)', () => {
  beforeEach(() => {
    mockedApi.get.mockReset()
    mockedApi.post.mockReset()
    mockedApi.put.mockReset()
  })

  test('business domains: returns res.data.data and maps page/page_size/keyword/enabled', async () => {
    const result = paged([{ id: 1, domain_code: 'hr', domain_name: 'HR', enabled: true, created_at: 1, updated_at: 1 }])
    mockedApi.get.mockResolvedValue({ data: { data: result } } as never)

    const out = await listBusinessDomains({ page: 2, page_size: 50, keyword: 'HR', enabled: true })

    expect(out).toEqual(result)
    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/business-domains', {
      params: { page: 2, page_size: 50, keyword: 'HR', enabled: true },
    })
  })

  test('drops undefined optional filters instead of sending them', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listBusinessDomains({ page: 1, page_size: 20, keyword: undefined, enabled: undefined })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/business-domains', {
      params: { page: 1, page_size: 20 },
    })
  })

  test('owner teams and usage teams hit their own endpoints with base query', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listOwnerTeams({ page: 1 })
    await listUsageTeams({ page: 1 })

    expect(mockedApi.get).toHaveBeenNthCalledWith(1, '/api/ai-governance/owner-teams', { params: { page: 1 } })
    expect(mockedApi.get).toHaveBeenNthCalledWith(2, '/api/ai-governance/usage-teams', { params: { page: 1 } })
  })

  test('principals: extra filters business_domain_id/usage_team_id pass through', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listPrincipals({ page: 1, business_domain_id: 3, usage_team_id: 4 })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/principals', {
      params: { page: 1, business_domain_id: 3, usage_team_id: 4 },
    })
  })

  test('credential purposes: purpose_type filter passes through', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listCredentialPurposes({ page: 1, purpose_type: 'IDE' })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/credential-purposes', {
      params: { page: 1, purpose_type: 'IDE' },
    })
  })

  test('applications: business_domain_id/owner_team_id filters pass through', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listApplications({ page: 1, business_domain_id: 3, owner_team_id: 5 })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/applications', {
      params: { page: 1, business_domain_id: 3, owner_team_id: 5 },
    })
  })

  test('identity profiles: identity_mode/token_id filters pass through', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listIdentityProfiles({ page: 1, identity_mode: 'STATIC', token_id: 7 })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/identity-profiles', {
      params: { page: 1, identity_mode: 'STATIC', token_id: 7 },
    })
  })
})

describe('identity profile detail aggregate DTO + {}→null normalization', () => {
  beforeEach(() => {
    mockedApi.get.mockReset()
  })

  test('getIdentityProfile returns the aggregate with real principal/purpose preserved', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: detail } } as never)

    const out = await getIdentityProfile(11)

    expect(out).toEqual(detail)
    expect(out.profile.id).toBe(11)
    expect(out.principal?.principal_code).toBe('p-3')
    expect(out.bindings[0].app_code).toBe('app-21')
    expect(out.risk.risk_level).toBe('LOWER_RISK')
    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/identity-profiles/11')
  })

  test('getIdentityProfile normalizes backend principal={} → null', async () => {
    const raw = { ...detail, principal: {} as never }
    mockedApi.get.mockResolvedValue({ data: { data: raw } } as never)

    const out = await getIdentityProfile(11)

    expect(out.principal).toBeNull()
    expect(out.purpose).toEqual(detail.purpose)
  })

  test('getIdentityProfile normalizes backend purpose={} → null', async () => {
    const raw = { ...detail, purpose: {} as never }
    mockedApi.get.mockResolvedValue({ data: { data: raw } } as never)

    const out = await getIdentityProfile(11)

    expect(out.purpose).toBeNull()
    expect(out.principal).toEqual(detail.principal)
  })

  test('listIdentityProfiles normalizes empty principal/purpose on every item', async () => {
    const rawA = { ...detail, id: 0, principal: {}, purpose: {} }
    const rawB = detail
    mockedApi.get.mockResolvedValue({ data: { data: paged([rawA, rawB]) } } as never)

    const out = await listIdentityProfiles({ page: 1 })

    expect(out.items[0].principal).toBeNull()
    expect(out.items[0].purpose).toBeNull()
    expect(out.items[1].principal).toEqual(detail.principal)
    expect(out.items[1].purpose).toEqual(detail.purpose)
  })
})

describe('write payload contract: no id/created_at/updated_at', () => {
  beforeEach(() => {
    mockedApi.post.mockReset()
    mockedApi.put.mockReset()
  })

  test('create payload carries only business fields', async () => {
    mockedApi.post.mockResolvedValue({
      data: { data: { id: 1, domain_code: 'hr', domain_name: 'HR', enabled: true, created_at: 1, updated_at: 1 } },
    } as never)

    await createBusinessDomain({ domain_code: 'hr', domain_name: 'HR' })

    const sent = mockedApi.post.mock.calls[0][1] as Record<string, unknown>
    expect(sent).toEqual({ domain_code: 'hr', domain_name: 'HR' })
    expect(sent).not.toHaveProperty('id')
    expect(sent).not.toHaveProperty('created_at')
    expect(sent).not.toHaveProperty('updated_at')
  })

  test('create identity profile returns a bare Profile, not the aggregate Detail', async () => {
    mockedApi.post.mockResolvedValue({ data: { data: bareProfile } } as never)

    const out = await createIdentityProfile({
      token_id: 7,
      identity_mode: 'STATIC',
      attribution_target_type: 'PRINCIPAL',
      identity_assurance: 'CREDENTIAL_ONLY',
      app_ids: [21, 22],
    })

    expect(out).toEqual(bareProfile)
    expect(out).not.toHaveProperty('bindings')
    expect(out).not.toHaveProperty('risk')
    expect(out).not.toHaveProperty('principal')

    const sent = mockedApi.post.mock.calls[0][1] as Record<string, unknown>
    expect(sent).toMatchObject({ token_id: 7, identity_mode: 'STATIC', app_ids: [21, 22] })
    expect(sent).not.toHaveProperty('id')
    expect(sent).not.toHaveProperty('created_at')
    expect(sent).not.toHaveProperty('updated_at')
  })

  test('update identity profile returns a bare Profile and never submits token_id', async () => {
    mockedApi.put.mockResolvedValue({ data: { data: bareProfile } } as never)

    const out = await updateIdentityProfile(11, {
      identity_assurance: 'SIGNED_CONTEXT',
      rate_limit_max_requests: 200,
    })

    expect(out).toEqual(bareProfile)
    expect(out).not.toHaveProperty('bindings')

    const sent = mockedApi.put.mock.calls[0][1] as Record<string, unknown>
    expect(sent).toMatchObject({ identity_assurance: 'SIGNED_CONTEXT', rate_limit_max_requests: 200 })
    expect(sent).not.toHaveProperty('token_id')
    expect(sent).not.toHaveProperty('id')
    expect(sent).not.toHaveProperty('created_at')
    expect(sent).not.toHaveProperty('updated_at')
  })

  test('replaceIdentityProfileAppBindings PUTs app_ids and returns the binding array', async () => {
    mockedApi.put.mockResolvedValue({ data: { data: bindingRows } } as never)

    const out = await replaceIdentityProfileAppBindings(11, { app_ids: [21, 22, 23] })

    expect(out).toEqual(bindingRows)
    expect(mockedApi.put).toHaveBeenCalledWith(
      '/api/ai-governance/identity-profiles/11/app-bindings',
      { app_ids: [21, 22, 23] }
    )
  })
})

describe('signing key contract', () => {
  beforeEach(() => {
    mockedApi.get.mockReset()
    mockedApi.post.mockReset()
  })

  test('listSigningKeys returns a plain metadata array (no secret)', async () => {
    const key = { id: 1, profile_id: 11, key_id: 'kid-1', status: 'ACTIVE' as const, not_before: 1, expires_at: 2, revoked_at: 0, created_at: 1, updated_at: 1 }
    mockedApi.get.mockResolvedValue({ data: { data: [key] } } as never)

    const out = await listSigningKeys(11)

    expect(out).toEqual([key])
    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/identity-profiles/11/signing-keys')
  })

  test('generate/rotate return {key, secret} from the one-time response', async () => {
    const issued = { key: { id: 1, profile_id: 11, key_id: 'kid-2', status: 'ACTIVE' as const, not_before: 1, expires_at: 2, revoked_at: 0, created_at: 1, updated_at: 1 }, secret: 'sk-abc' }
    mockedApi.post.mockResolvedValue({ data: { data: issued } } as never)

    await expect(generateSigningKey(11)).resolves.toEqual(issued)
    await expect(rotateSigningKey(11)).resolves.toEqual(issued)
    expect(mockedApi.post).toHaveBeenNthCalledWith(1, '/api/ai-governance/identity-profiles/11/signing-keys/generate')
    expect(mockedApi.post).toHaveBeenNthCalledWith(2, '/api/ai-governance/identity-profiles/11/signing-keys/rotate')
  })

  test('revoke returns {revoked:true}', async () => {
    mockedApi.post.mockResolvedValue({ data: { data: { revoked: true } } } as never)

    await expect(revokeSigningKey(11, 'kid-1')).resolves.toEqual({ revoked: true })
    expect(mockedApi.post).toHaveBeenCalledWith('/api/ai-governance/identity-profiles/11/signing-keys/kid-1/revoke')
  })
})

describe('identity audit contract', () => {
  beforeEach(() => {
    mockedApi.get.mockReset()
  })

  test('passes only the real backend filters (page/page_size/request_id/token_id/profile_id/result/reason_code)', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listIdentityAuditEvents({
      page: 1,
      page_size: 50,
      request_id: 'req-1',
      token_id: 7,
      profile_id: 9,
      result: 'REJECTED',
      reason_code: 'NO_PRINCIPAL',
    })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/identity-audit-events', {
      params: {
        page: 1,
        page_size: 50,
        request_id: 'req-1',
        token_id: 7,
        profile_id: 9,
        result: 'REJECTED',
        reason_code: 'NO_PRINCIPAL',
      },
    })
  })

  test('omits every optional filter when absent', async () => {
    mockedApi.get.mockResolvedValue({ data: { data: paged([]) } } as never)

    await listIdentityAuditEvents({ page: 1, page_size: 20 })

    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/identity-audit-events', {
      params: { page: 1, page_size: 20 },
    })
  })

  test('audit events allow unresolved Profile: identity_mode/identity_assurance are empty strings', async () => {
    const unresolved: GovernanceAuditEvent = {
      ...auditEvent,
      identity_mode: '',
      identity_assurance: '',
    }
    mockedApi.get.mockResolvedValue({ data: { data: paged([unresolved]) } } as never)

    const out = await listIdentityAuditEvents({ page: 1, page_size: 20 })

    expect(out.items).toEqual([unresolved])
    expect(out.items[0].identity_mode).toBe('')
    expect(out.items[0].identity_assurance).toBe('')
  })
})

describe('enterprise usage contract (§12)', () => {
  test('listEnterpriseUsage returns a bare array (not PagedResult) and forwards snake_case filters', async () => {
    const rows = [{ id: 1, bucket_time: 1, profile_id: 1, principal_id: 1, credential_purpose_id: 1, usage_business_domain_id: 1, usage_team_id: 1, caller_key: 'c', root_app_code: 'r', app_id: 1, app_business_domain_id: 1, owner_team_id: 1, identity_assurance: 'UNVERIFIED', client_verified: false, model_name: 'gpt-4', dimension_hash: 'h', request_count: 1, success_count: 1, error_count: 0, input_tokens: 10, output_tokens: 20, total_tokens: 30, quota_net: 5, duration_ms_total: 100, created_at: 1, updated_at: 1 }]
    mockedApi.get.mockResolvedValue({ data: { data: rows } } as never)

    const out = await listEnterpriseUsage({ bucket_start: 1, bucket_end: 2, identity_assurance: 'UNVERIFIED', model_name: 'gpt-4' })

    expect(out).toEqual(rows)
    expect(out).not.toHaveProperty('total')
    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/usage/stats', {
      params: { bucket_start: 1, bucket_end: 2, identity_assurance: 'UNVERIFIED', model_name: 'gpt-4' },
    })
  })

  test('anomalies forwards bucket_start/bucket_end and identity_assurance is a strong-typed fixture', async () => {
    const anomaly: EnterpriseUsageAnomaly = {
      profile_id: 1,
      principal_id: 1,
      credential_purpose_id: 1,
      bucket_time: 1,
      metric: 'request',
      current: 10,
      baseline: 5,
      threshold: 1.5,
      model_name: 'gpt-4',
      identity_assurance: 'SIGNED_CONTEXT',
    }
    mockedApi.get.mockResolvedValue({ data: { data: [anomaly] } } as never)

    const out = await listEnterpriseUsageAnomalies(100, 200)

    expect(out).toEqual([anomaly])
    expect(mockedApi.get).toHaveBeenCalledWith('/api/ai-governance/usage/anomalies', {
      params: { bucket_start: 100, bucket_end: 200 },
    })
  })

  test('rebuild POSTs start/end as query params and returns processed_logs', async () => {
    mockedApi.post.mockResolvedValue({ data: { data: { processed_logs: 42 } } } as never)

    const out = await rebuildEnterpriseUsage(100, 200)

    expect(out).toEqual({ processed_logs: 42 })
    expect(mockedApi.post).toHaveBeenCalledWith(
      '/api/ai-governance/usage/rebuild',
      null,
      { params: { start: 100, end: 200 } }
    )
  })
})

describe('governance overview section registration', () => {
  test('registers exactly the 9 frozen sections in order, each with meta', () => {
    expect(AI_GOVERNANCE_SECTION_IDS).toEqual([
      'business-domains',
      'usage-teams',
      'principals',
      'credential-purposes',
      'owner-teams',
      'applications',
      'identity-profiles',
      'identity-audit',
      'usage',
    ])
    expect(AI_GOVERNANCE_SECTION_IDS).toHaveLength(9)
    expect(Object.keys(AI_GOVERNANCE_SECTION_META)).toHaveLength(9)
    for (const id of AI_GOVERNANCE_SECTION_IDS) {
      expect(AI_GOVERNANCE_SECTION_META[id]).toBeDefined()
    }
  })
})
