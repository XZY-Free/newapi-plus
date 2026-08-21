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
 * 企业 AI 治理 API 封装层。
 *
 * 唯一访问后端 `/api/ai-governance/*` 的边界。全部接口均要求 Root 权限（后端已校验）。
 *
 * 契约约束：
 * - 列表统一返回 {@link PagedResult}（items/total/page/page_size）。
 * - Query 参数显式使用后端 snake_case 键；可选筛选通过 {@link pickDefined} 剔除
 *   undefined 后透传，不依赖 Axios 自动转换。
 * - 写请求使用显式 Create/Update Payload 类型，禁止提交 id/created_at/updated_at。
 */
import { api } from '@/lib/api'

import type {
  ApiResponse,
  ApplicationListQuery,
  BusinessDomainListQuery,
  CredentialPurposeListQuery,
  CreateApplicationPayload,
  CreateBusinessDomainPayload,
  CreateCredentialPurposePayload,
  CreateIdentityProfilePayload,
  CreateOwnerTeamPayload,
  CreatePrincipalPayload,
  CreateUsageTeamPayload,
  EnterpriseUsageAnomaly,
  EnterpriseUsageFilter,
  EnterpriseUsageRow,
  GovernanceApplication,
  GovernanceAuditEvent,
  GovernanceBusinessDomain,
  GovernanceCredentialPurpose,
  GovernanceIdentityProfileDetail,
  GovernanceOwnerTeam,
  GovernancePrincipal,
  GovernanceSigningKey,
  GovernanceSigningKeyIssued,
  GovernanceSigningKeyRevokeResponse,
  GovernanceUsageTeam,
  IdentityAuditListQuery,
  IdentityProfileListQuery,
  OwnerTeamListQuery,
  PagedResult,
  PrincipalListQuery,
  ReplaceAppBindingsPayload,
  UpdateApplicationPayload,
  UpdateBusinessDomainPayload,
  UpdateCredentialPurposePayload,
  UpdateIdentityProfilePayload,
  UpdateOwnerTeamPayload,
  UpdatePrincipalPayload,
  UpdateUsageTeamPayload,
  UsageRebuildResponse,
  UsageTeamListQuery,
} from './types'

/**
 * 显式构建查询参数：剔除值为 undefined 的键。
 * 这是 API 边界处唯一允许的“参数清洗”，保证可选筛选不会被误发成空串/0。
 */
export function pickDefined<T extends object>(obj: T) {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(obj)) {
    if (v !== undefined) out[k] = v
  }
  return out
}

// ---------------------------------------------------------------------------
// 业务领域
// ---------------------------------------------------------------------------

export async function listBusinessDomains(
  query: BusinessDomainListQuery = {}
): Promise<PagedResult<GovernanceBusinessDomain>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceBusinessDomain>>>(
    '/api/ai-governance/business-domains',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function createBusinessDomain(
  payload: CreateBusinessDomainPayload
): Promise<GovernanceBusinessDomain> {
  const res = await api.post<ApiResponse<GovernanceBusinessDomain>>(
    '/api/ai-governance/business-domains',
    payload
  )
  return res.data.data
}

export async function updateBusinessDomain(
  id: number,
  payload: UpdateBusinessDomainPayload
): Promise<GovernanceBusinessDomain> {
  const res = await api.put<ApiResponse<GovernanceBusinessDomain>>(
    `/api/ai-governance/business-domains/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 应用团队 / 使用团队
// ---------------------------------------------------------------------------

export async function listOwnerTeams(
  query: OwnerTeamListQuery = {}
): Promise<PagedResult<GovernanceOwnerTeam>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceOwnerTeam>>>(
    '/api/ai-governance/owner-teams',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function createOwnerTeam(
  payload: CreateOwnerTeamPayload
): Promise<GovernanceOwnerTeam> {
  const res = await api.post<ApiResponse<GovernanceOwnerTeam>>(
    '/api/ai-governance/owner-teams',
    payload
  )
  return res.data.data
}

export async function updateOwnerTeam(
  id: number,
  payload: UpdateOwnerTeamPayload
): Promise<GovernanceOwnerTeam> {
  const res = await api.put<ApiResponse<GovernanceOwnerTeam>>(
    `/api/ai-governance/owner-teams/${id}`,
    payload
  )
  return res.data.data
}

export async function listUsageTeams(
  query: UsageTeamListQuery = {}
): Promise<PagedResult<GovernanceUsageTeam>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceUsageTeam>>>(
    '/api/ai-governance/usage-teams',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function createUsageTeam(
  payload: CreateUsageTeamPayload
): Promise<GovernanceUsageTeam> {
  const res = await api.post<ApiResponse<GovernanceUsageTeam>>(
    '/api/ai-governance/usage-teams',
    payload
  )
  return res.data.data
}

export async function updateUsageTeam(
  id: number,
  payload: UpdateUsageTeamPayload
): Promise<GovernanceUsageTeam> {
  const res = await api.put<ApiResponse<GovernanceUsageTeam>>(
    `/api/ai-governance/usage-teams/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 使用主体 / 凭证用途
// ---------------------------------------------------------------------------

export async function listPrincipals(
  query: PrincipalListQuery = {}
): Promise<PagedResult<GovernancePrincipal>> {
  const res = await api.get<ApiResponse<PagedResult<GovernancePrincipal>>>(
    '/api/ai-governance/principals',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function getPrincipal(id: number): Promise<GovernancePrincipal> {
  const res = await api.get<ApiResponse<GovernancePrincipal>>(
    `/api/ai-governance/principals/${id}`
  )
  return res.data.data
}

export async function createPrincipal(
  payload: CreatePrincipalPayload
): Promise<GovernancePrincipal> {
  const res = await api.post<ApiResponse<GovernancePrincipal>>(
    '/api/ai-governance/principals',
    payload
  )
  return res.data.data
}

export async function updatePrincipal(
  id: number,
  payload: UpdatePrincipalPayload
): Promise<GovernancePrincipal> {
  const res = await api.put<ApiResponse<GovernancePrincipal>>(
    `/api/ai-governance/principals/${id}`,
    payload
  )
  return res.data.data
}

export async function listCredentialPurposes(
  query: CredentialPurposeListQuery = {}
): Promise<PagedResult<GovernanceCredentialPurpose>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceCredentialPurpose>>>(
    '/api/ai-governance/credential-purposes',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function createCredentialPurpose(
  payload: CreateCredentialPurposePayload
): Promise<GovernanceCredentialPurpose> {
  const res = await api.post<ApiResponse<GovernanceCredentialPurpose>>(
    '/api/ai-governance/credential-purposes',
    payload
  )
  return res.data.data
}

export async function updateCredentialPurpose(
  id: number,
  payload: UpdateCredentialPurposePayload
): Promise<GovernanceCredentialPurpose> {
  const res = await api.put<ApiResponse<GovernanceCredentialPurpose>>(
    `/api/ai-governance/credential-purposes/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// AI 应用
// ---------------------------------------------------------------------------

export async function listApplications(
  query: ApplicationListQuery = {}
): Promise<PagedResult<GovernanceApplication>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceApplication>>>(
    '/api/ai-governance/applications',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function getApplication(id: number): Promise<GovernanceApplication> {
  const res = await api.get<ApiResponse<GovernanceApplication>>(
    `/api/ai-governance/applications/${id}`
  )
  return res.data.data
}

export async function createApplication(
  payload: CreateApplicationPayload
): Promise<GovernanceApplication> {
  const res = await api.post<ApiResponse<GovernanceApplication>>(
    '/api/ai-governance/applications',
    payload
  )
  return res.data.data
}

export async function updateApplication(
  id: number,
  payload: UpdateApplicationPayload
): Promise<GovernanceApplication> {
  const res = await api.put<ApiResponse<GovernanceApplication>>(
    `/api/ai-governance/applications/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// API Key 身份（列表与详情均返回聚合详情 DTO）
// ---------------------------------------------------------------------------

export async function listIdentityProfiles(
  query: IdentityProfileListQuery = {}
): Promise<PagedResult<GovernanceIdentityProfileDetail>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceIdentityProfileDetail>>>(
    '/api/ai-governance/identity-profiles',
    { params: pickDefined(query) }
  )
  return res.data.data
}

export async function getIdentityProfile(
  id: number
): Promise<GovernanceIdentityProfileDetail> {
  const res = await api.get<ApiResponse<GovernanceIdentityProfileDetail>>(
    `/api/ai-governance/identity-profiles/${id}`
  )
  return res.data.data
}

export async function createIdentityProfile(
  payload: CreateIdentityProfilePayload
): Promise<GovernanceIdentityProfileDetail> {
  const res = await api.post<ApiResponse<GovernanceIdentityProfileDetail>>(
    '/api/ai-governance/identity-profiles',
    payload
  )
  return res.data.data
}

export async function updateIdentityProfile(
  id: number,
  payload: UpdateIdentityProfilePayload
): Promise<GovernanceIdentityProfileDetail> {
  const res = await api.put<ApiResponse<GovernanceIdentityProfileDetail>>(
    `/api/ai-governance/identity-profiles/${id}`,
    payload
  )
  return res.data.data
}

export async function replaceIdentityProfileAppBindings(
  id: number,
  payload: ReplaceAppBindingsPayload
): Promise<unknown> {
  const res = await api.put<ApiResponse<unknown>>(
    `/api/ai-governance/identity-profiles/${id}/app-bindings`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 签名密钥
// ---------------------------------------------------------------------------

export async function listSigningKeys(
  profileId: number
): Promise<GovernanceSigningKey[]> {
  const res = await api.get<ApiResponse<GovernanceSigningKey[]>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys`
  )
  return res.data.data
}

export async function generateSigningKey(
  profileId: number
): Promise<GovernanceSigningKeyIssued> {
  const res = await api.post<ApiResponse<GovernanceSigningKeyIssued>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys/generate`
  )
  return res.data.data
}

export async function rotateSigningKey(
  profileId: number
): Promise<GovernanceSigningKeyIssued> {
  const res = await api.post<ApiResponse<GovernanceSigningKeyIssued>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys/rotate`
  )
  return res.data.data
}

export async function revokeSigningKey(
  profileId: number,
  keyId: string
): Promise<GovernanceSigningKeyRevokeResponse> {
  const res = await api.post<ApiResponse<GovernanceSigningKeyRevokeResponse>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys/${encodeURIComponent(keyId)}/revoke`
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 身份审计
// ---------------------------------------------------------------------------

export async function listIdentityAuditEvents(
  query: IdentityAuditListQuery = {}
): Promise<PagedResult<GovernanceAuditEvent>> {
  const res = await api.get<ApiResponse<PagedResult<GovernanceAuditEvent>>>(
    '/api/ai-governance/identity-audit-events',
    { params: pickDefined(query) }
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 企业用量（§12：stats 返回裸数组，非分页）
// ---------------------------------------------------------------------------

export async function listEnterpriseUsage(
  filter: EnterpriseUsageFilter = {}
): Promise<EnterpriseUsageRow[]> {
  const res = await api.get<ApiResponse<EnterpriseUsageRow[]>>(
    '/api/ai-governance/usage/stats',
    { params: pickDefined(filter) }
  )
  return res.data.data
}

export async function listEnterpriseUsageAnomalies(
  bucketStart: number,
  bucketEnd: number
): Promise<EnterpriseUsageAnomaly[]> {
  const res = await api.get<ApiResponse<EnterpriseUsageAnomaly[]>>(
    '/api/ai-governance/usage/anomalies',
    { params: pickDefined({ bucket_start: bucketStart, bucket_end: bucketEnd }) }
  )
  return res.data.data
}

export async function rebuildEnterpriseUsage(
  start: number,
  end: number
): Promise<UsageRebuildResponse> {
  const res = await api.post<ApiResponse<UsageRebuildResponse>>(
    '/api/ai-governance/usage/rebuild',
    null,
    { params: pickDefined({ start, end }) }
  )
  return res.data.data
}
