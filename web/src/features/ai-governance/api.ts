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
 * 唯一访问后端 `/api/ai-governance/*` 的边界。后续子批（§11-B～E）页面一律通过
 * 本模块调用，不得在组件内直接拼 URL。全部接口均要求 Root 权限（后端已校验）。
 */
import { api } from '@/lib/api'

import type {
  ApiResponse,
  EnterpriseUsageAnomaly,
  EnterpriseUsageFilter,
  EnterpriseUsageRow,
  GovernanceApplication,
  GovernanceAuditEvent,
  GovernanceBusinessDomain,
  GovernanceCredentialPurpose,
  GovernanceIdentityAppBinding,
  GovernanceIdentityProfile,
  GovernanceOwnerTeam,
  GovernancePrincipal,
  GovernanceSigningKey,
  GovernanceUsageTeam,
  UsageRebuildResponse,
} from './types'

// ---------------------------------------------------------------------------
// 业务领域
// ---------------------------------------------------------------------------

export async function listBusinessDomains() {
  const res = await api.get<ApiResponse<GovernanceBusinessDomain[]>>(
    '/api/ai-governance/business-domains'
  )
  return res.data.data
}

export async function createBusinessDomain(payload: {
  domain_code: string
  domain_name: string
  enabled?: boolean
}) {
  const res = await api.post<ApiResponse<GovernanceBusinessDomain>>(
    '/api/ai-governance/business-domains',
    payload
  )
  return res.data.data
}

export async function updateBusinessDomain(
  id: number,
  payload: Partial<GovernanceBusinessDomain>
) {
  const res = await api.put<ApiResponse<GovernanceBusinessDomain>>(
    `/api/ai-governance/business-domains/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 应用团队 / 使用团队
// ---------------------------------------------------------------------------

export async function listOwnerTeams() {
  const res = await api.get<ApiResponse<GovernanceOwnerTeam[]>>(
    '/api/ai-governance/owner-teams'
  )
  return res.data.data
}

export async function createOwnerTeam(payload: {
  team_code: string
  team_name: string
  enabled?: boolean
}) {
  const res = await api.post<ApiResponse<GovernanceOwnerTeam>>(
    '/api/ai-governance/owner-teams',
    payload
  )
  return res.data.data
}

export async function updateOwnerTeam(
  id: number,
  payload: Partial<GovernanceOwnerTeam>
) {
  const res = await api.put<ApiResponse<GovernanceOwnerTeam>>(
    `/api/ai-governance/owner-teams/${id}`,
    payload
  )
  return res.data.data
}

export async function listUsageTeams() {
  const res = await api.get<ApiResponse<GovernanceUsageTeam[]>>(
    '/api/ai-governance/usage-teams'
  )
  return res.data.data
}

export async function createUsageTeam(payload: {
  team_code: string
  team_name: string
  enabled?: boolean
}) {
  const res = await api.post<ApiResponse<GovernanceUsageTeam>>(
    '/api/ai-governance/usage-teams',
    payload
  )
  return res.data.data
}

export async function updateUsageTeam(
  id: number,
  payload: Partial<GovernanceUsageTeam>
) {
  const res = await api.put<ApiResponse<GovernanceUsageTeam>>(
    `/api/ai-governance/usage-teams/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 使用主体 / 凭证用途
// ---------------------------------------------------------------------------

export async function listPrincipals() {
  const res = await api.get<ApiResponse<GovernancePrincipal[]>>(
    '/api/ai-governance/principals'
  )
  return res.data.data
}

export async function getPrincipal(id: number) {
  const res = await api.get<ApiResponse<GovernancePrincipal>>(
    `/api/ai-governance/principals/${id}`
  )
  return res.data.data
}

export async function createPrincipal(payload: Partial<GovernancePrincipal>) {
  const res = await api.post<ApiResponse<GovernancePrincipal>>(
    '/api/ai-governance/principals',
    payload
  )
  return res.data.data
}

export async function updatePrincipal(
  id: number,
  payload: Partial<GovernancePrincipal>
) {
  const res = await api.put<ApiResponse<GovernancePrincipal>>(
    `/api/ai-governance/principals/${id}`,
    payload
  )
  return res.data.data
}

export async function listCredentialPurposes() {
  const res = await api.get<ApiResponse<GovernanceCredentialPurpose[]>>(
    '/api/ai-governance/credential-purposes'
  )
  return res.data.data
}

export async function createCredentialPurpose(payload: {
  purpose_code: string
  purpose_name: string
  purpose_type: string
  enabled?: boolean
}) {
  const res = await api.post<ApiResponse<GovernanceCredentialPurpose>>(
    '/api/ai-governance/credential-purposes',
    payload
  )
  return res.data.data
}

export async function updateCredentialPurpose(
  id: number,
  payload: Partial<GovernanceCredentialPurpose>
) {
  const res = await api.put<ApiResponse<GovernanceCredentialPurpose>>(
    `/api/ai-governance/credential-purposes/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// AI 应用
// ---------------------------------------------------------------------------

export async function listApplications() {
  const res = await api.get<ApiResponse<GovernanceApplication[]>>(
    '/api/ai-governance/applications'
  )
  return res.data.data
}

export async function getApplication(id: number) {
  const res = await api.get<ApiResponse<GovernanceApplication>>(
    `/api/ai-governance/applications/${id}`
  )
  return res.data.data
}

export async function createApplication(payload: Partial<GovernanceApplication>) {
  const res = await api.post<ApiResponse<GovernanceApplication>>(
    '/api/ai-governance/applications',
    payload
  )
  return res.data.data
}

export async function updateApplication(
  id: number,
  payload: Partial<GovernanceApplication>
) {
  const res = await api.put<ApiResponse<GovernanceApplication>>(
    `/api/ai-governance/applications/${id}`,
    payload
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// API Key 身份
// ---------------------------------------------------------------------------

export async function listIdentityProfiles() {
  const res = await api.get<ApiResponse<GovernanceIdentityProfile[]>>(
    '/api/ai-governance/identity-profiles'
  )
  return res.data.data
}

export async function getIdentityProfile(id: number) {
  const res = await api.get<ApiResponse<GovernanceIdentityProfile>>(
    `/api/ai-governance/identity-profiles/${id}`
  )
  return res.data.data
}

export async function createIdentityProfile(
  payload: Partial<GovernanceIdentityProfile>
) {
  const res = await api.post<ApiResponse<GovernanceIdentityProfile>>(
    '/api/ai-governance/identity-profiles',
    payload
  )
  return res.data.data
}

export async function updateIdentityProfile(
  id: number,
  payload: Partial<GovernanceIdentityProfile>
) {
  const res = await api.put<ApiResponse<GovernanceIdentityProfile>>(
    `/api/ai-governance/identity-profiles/${id}`,
    payload
  )
  return res.data.data
}

export async function replaceIdentityProfileAppBindings(
  id: number,
  bindings: { app_id: number; enabled: boolean }[]
) {
  const res = await api.put<ApiResponse<GovernanceIdentityAppBinding[]>>(
    `/api/ai-governance/identity-profiles/${id}/app-bindings`,
    bindings
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 签名密钥
// ---------------------------------------------------------------------------

export async function listSigningKeys(profileId: number) {
  const res = await api.get<ApiResponse<GovernanceSigningKey[]>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys`
  )
  return res.data.data
}

export async function generateSigningKey(profileId: number) {
  const res = await api.post<ApiResponse<{ secret: string; key: GovernanceSigningKey }>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys/generate`
  )
  return res.data.data
}

export async function rotateSigningKey(profileId: number) {
  const res = await api.post<ApiResponse<{ secret: string; key: GovernanceSigningKey }>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys/rotate`
  )
  return res.data.data
}

export async function revokeSigningKey(profileId: number, keyId: string) {
  const res = await api.post<ApiResponse<GovernanceSigningKey>>(
    `/api/ai-governance/identity-profiles/${profileId}/signing-keys/${encodeURIComponent(keyId)}/revoke`
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 身份审计
// ---------------------------------------------------------------------------

export async function listIdentityAuditEvents(params?: {
  startTime?: number
  endTime?: number
  reasonCode?: string
  result?: string
  page?: number
  pageSize?: number
}) {
  const res = await api.get<ApiResponse<GovernanceAuditEvent[]>>(
    '/api/ai-governance/identity-audit-events',
    { params }
  )
  return res.data.data
}

// ---------------------------------------------------------------------------
// 企业用量（对接 §12）
// ---------------------------------------------------------------------------

export async function listEnterpriseUsage(filter: EnterpriseUsageFilter = {}) {
  const res = await api.get<ApiResponse<EnterpriseUsageRow[]>>(
    '/api/ai-governance/usage/stats',
    { params: filter }
  )
  return res.data.data
}

export async function listEnterpriseUsageAnomalies(
  bucketStart: number,
  bucketEnd: number
) {
  const res = await api.get<ApiResponse<EnterpriseUsageAnomaly[]>>(
    '/api/ai-governance/usage/anomalies',
    { params: { bucket_start: bucketStart, bucket_end: bucketEnd } }
  )
  return res.data.data
}

export async function rebuildEnterpriseUsage(start: number, end: number) {
  const res = await api.post<ApiResponse<UsageRebuildResponse>>(
    '/api/ai-governance/usage/rebuild',
    null,
    { params: { start, end } }
  )
  return res.data.data
}
