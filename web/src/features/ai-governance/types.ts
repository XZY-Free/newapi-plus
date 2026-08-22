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
 * 企业 AI 治理公共领域模型（V1.1 §11 / §12）。
 *
 * 本文件是前端对后端 `/api/ai-governance/*` 契约的唯一事实来源，字段命名与
 * 后端 model 的 JSON tag 及 Controller 请求/响应结构一一对应。
 *
 * 本文件刻意区分两类类型：
 * - `*Entity`：后端返回的“事实”（含 id/created_at/updated_at 等只读字段）。
 * - `*Payload`：允许客户端写入的 Create / Update 请求体，绝不含 id、
 *   created_at、updated_at 等后端不接受的自增/审计字段。
 *
 * 列表响应统一为 {@link PagedResult}（后端 `listResult`：items/total/page/page_size）。
 */

// ---------------------------------------------------------------------------
// 通用响应包装与分页
// ---------------------------------------------------------------------------

export interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}

/**
 * 治理列表统一分页响应（后端 `listResult`，见 controller/ai_governance.go）。
 */
export interface PagedResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

/** 所有治理列表共用的分页/关键字/启停筛选（page/page_size 后端有 1/20 默认）。 */
export interface GovernanceListQuery {
  page?: number
  page_size?: number
  keyword?: string
  enabled?: boolean
}

// ---------------------------------------------------------------------------
// 枚举（强类型，与 constant/ai_attribution.go 一一对应）
// ---------------------------------------------------------------------------

/** 使用主体类型。第一阶段后端固定为 PERSON，UI 不得允许自由输入。 */
export type PrincipalType = 'PERSON'

/** 凭证登记用途类型。 */
export type CredentialPurposeType =
  | 'DESKTOP_CLIENT'
  | 'IDE'
  | 'SCRIPT'
  | 'SERVICE'
  | 'OTHER'

/** 身份取得方式。 */
export type IdentityMode = 'STATIC' | 'DYNAMIC' | 'HYBRID'

/** 归因目标。 */
export type AttributionTargetType = 'PRINCIPAL' | 'APPLICATION' | 'PLATFORM'

/** 身份可信等级。 */
export type IdentityAssurance =
  | 'CREDENTIAL_ONLY'
  | 'SIGNED_CONTEXT'
  | 'HYBRID_VERIFIED_CONTEXT'

/**
 * 审计事件专用身份取得方式：允许 “Profile 尚未解析出” 的空串态。
 * 仅用于审计事件，不得污染 Identity Profile 配置所允许的三种值。
 */
export type AuditIdentityMode = IdentityMode | ''

/**
 * 审计事件专用身份可信等级：允许 “Profile 尚未解析出” 的空串态。
 * 仅用于审计事件，不得污染 Identity Profile 配置所允许的三种值。
 */
export type AuditIdentityAssurance = IdentityAssurance | ''

/** 签名密钥状态。 */
export type SigningKeyStatus = 'ACTIVE' | 'RETIRING' | 'REVOKED'

/** 弱身份凭证安全姿态风险等级（types.RiskPosture.risk_level）。 */
export type CredentialRiskLevel = 'LOWER_RISK' | 'MEDIUM_RISK' | 'HIGH_RISK'

/**
 * 用量投影中的身份可信等级（§12）：额外允许 UNVERIFIED。
 * 仅用于用量领域，不得污染 Identity Profile 自身允许的三种配置值。
 */
export type UsageIdentityAssurance = IdentityAssurance | 'UNVERIFIED'

/** 身份审计结果（constant.IdentityAuditResult*）。 */
export type IdentityAuditResult = 'UNVERIFIED' | 'REJECTED'

// ---------------------------------------------------------------------------
// 主数据实体（后端返回）
// ---------------------------------------------------------------------------

export interface GovernanceBusinessDomain {
  id: number
  domain_code: string
  domain_name: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface GovernanceOwnerTeam {
  id: number
  team_code: string
  team_name: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface GovernanceUsageTeam {
  id: number
  team_code: string
  team_name: string
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface GovernancePrincipal {
  id: number
  principal_code: string
  principal_name: string
  principal_type: PrincipalType
  business_domain_id: number
  usage_team_id: number
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface GovernanceCredentialPurpose {
  id: number
  purpose_code: string
  purpose_name: string
  purpose_type: CredentialPurposeType
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface GovernanceApplication {
  id: number
  app_code: string
  app_name: string
  business_domain_id: number
  owner_team_id: number
  enabled: boolean
  created_at: number
  updated_at: number
}

// ---------------------------------------------------------------------------
// Identity Profile 聚合详情（列表与详情复用后端 buildIdentityProfileDetail）
// ---------------------------------------------------------------------------

/** 聚合详情中的 Principal 摘要。后端 principal_id<=0 时返回空对象。 */
export interface GovernancePrincipalSummary {
  principal_id: number
  principal_code: string
  principal_name: string
}

/** 聚合详情中的 Credential Purpose 摘要。后端 credential_purpose_id<=0 时返回空对象。 */
export interface GovernancePurposeSummary {
  credential_purpose_id: number
  credential_purpose_code: string
  credential_purpose_name: string
}

/** 聚合详情中的 NewAPI Token 安全元数据（绝不含 Token Key 明文）。 */
export interface GovernanceTokenMeta {
  token_id: number
  token_name: string
  status: number
  expired_time: number
  unlimited: boolean
  ip_restricted: boolean
  model_restricted: boolean
  remain_quota: number
  created_time: number
}

/** 聚合详情中一个已绑定应用的视图（含 domain / owner team 快照）。 */
export interface GovernanceBindingView {
  id: number
  app_id: number
  enabled: boolean
  app_code: string
  app_name: string
  business_domain_id?: number
  business_domain_code?: string
  business_domain_name?: string
  owner_team_id?: number
  owner_team_code?: string
  owner_team_name?: string
}

/** Profile 基础配置（聚合详情.profile 子对象，等同 ai_identity_profiles 行）。 */
export interface GovernanceIdentityProfile {
  id: number
  token_id: number
  identity_mode: IdentityMode
  attribution_target_type: AttributionTargetType
  identity_assurance: IdentityAssurance
  caller_id: string
  caller_name: string
  principal_id: number
  credential_purpose_id: number
  environment: string
  rate_limit_enabled: boolean
  rate_limit_window_seconds: number
  rate_limit_max_requests: number
  enabled: boolean
  created_at: number
  updated_at: number
}

/** 弱身份凭证安全姿态（types.RiskPosture）。 */
export interface GovernanceRiskPosture {
  ip_restricted: boolean
  model_restricted: boolean
  quota_restricted: boolean
  expiry_configured: boolean
  rate_limit_enabled: boolean
  credential_only: boolean
  rotation_overdue: boolean
  rotation_overdue_days?: number
  risk_level: CredentialRiskLevel
}

/**
 * App Binding 行（后端 `ai_identity_profile_app_bindings`）。
 * `replaceIdentityProfileAppBindings` 整体替换后返回该数组。
 */
export interface GovernanceIdentityAppBinding {
  id: number
  profile_id: number
  app_id: number
  enabled: boolean
  created_at: number
  updated_at: number
}

/** Profile 级请求频率限制配置。 */
export interface GovernanceRateLimitConfig {
  enabled: boolean
  window_seconds: number
  max_requests: number
}

/**
 * Identity Profile 聚合详情 DTO（后端 `buildIdentityProfileDetail`）。
 * 列表接口 `GET /identity-profiles` 与详情 `GET /identity-profiles/:id` 均返回本结构。
 */
export interface GovernanceIdentityProfileDetail {
  profile: GovernanceIdentityProfile
  principal: GovernancePrincipalSummary | null
  purpose: GovernancePurposeSummary | null
  token: GovernanceTokenMeta
  bindings: GovernanceBindingView[]
  risk: GovernanceRiskPosture
  rate_limits: {
    items: never[]
    config: GovernanceRateLimitConfig
  }
}

// ---------------------------------------------------------------------------
// 签名密钥
// ---------------------------------------------------------------------------

/** 签名密钥元数据视图（后端 signingKeyMetaView，绝不包含 secret/secret_ciphertext）。 */
export interface GovernanceSigningKey {
  id: number
  profile_id: number
  key_id: string
  status: SigningKeyStatus
  not_before: number
  expires_at: number
  revoked_at: number
  created_at: number
  updated_at: number
}

/** generate / rotate 响应：明文 secret 只在这一次响应中出现。 */
export interface GovernanceSigningKeyIssued {
  key: GovernanceSigningKey
  secret: string
}

/** revoke 响应。 */
export interface GovernanceSigningKeyRevokeResponse {
  revoked: true
}

// ---------------------------------------------------------------------------
// 身份审计事件
// ---------------------------------------------------------------------------

export interface GovernanceAuditEvent {
  id: number
  created_at: number
  request_id: string
  token_id: number
  profile_id: number
  caller_id: string
  principal_id: number
  credential_purpose_id: number
  identity_mode: AuditIdentityMode
  identity_assurance: AuditIdentityAssurance
  result: IdentityAuditResult
  reason_code: string
  claimed_root_app_id: string
  http_method: string
  request_path: string
  client_ip: string
}

// ---------------------------------------------------------------------------
// 写请求 Payload（与后端 Controller 请求结构严格一致，不含自增/审计字段）
// ---------------------------------------------------------------------------

// Business Domain
export interface CreateBusinessDomainPayload {
  domain_code: string
  domain_name: string
}
export interface UpdateBusinessDomainPayload {
  domain_name?: string
  enabled?: boolean
}

// Owner Team
export interface CreateOwnerTeamPayload {
  team_code: string
  team_name: string
}
export interface UpdateOwnerTeamPayload {
  team_name?: string
  enabled?: boolean
}

// Usage Team
export interface CreateUsageTeamPayload {
  team_code: string
  team_name: string
}
export interface UpdateUsageTeamPayload {
  team_name?: string
  enabled?: boolean
}

// Principal
export interface CreatePrincipalPayload {
  principal_code: string
  principal_name: string
  business_domain_id: number
  usage_team_id: number
}
export interface UpdatePrincipalPayload {
  principal_name?: string
  business_domain_id?: number
  usage_team_id?: number
  enabled?: boolean
}

// Credential Purpose
export interface CreateCredentialPurposePayload {
  purpose_code: string
  purpose_name: string
  purpose_type: CredentialPurposeType
}
export interface UpdateCredentialPurposePayload {
  purpose_name?: string
  purpose_type?: CredentialPurposeType
  enabled?: boolean
}

// AI Application
export interface CreateApplicationPayload {
  app_code: string
  app_name: string
  business_domain_id: number
  owner_team_id: number
}
export interface UpdateApplicationPayload {
  app_name?: string
  business_domain_id?: number
  owner_team_id?: number
  enabled?: boolean
}

// Identity Profile
/**
 * Profile 创建请求体。后端（controller identityProfileRequest）将 enabled 强制置 false；
 * 因此创建后必须完成绑定/签名/安全配置，再通过 Update 显式启用。
 */
export interface CreateIdentityProfilePayload {
  token_id: number
  identity_mode: IdentityMode
  attribution_target_type: AttributionTargetType
  identity_assurance: IdentityAssurance
  caller_id?: string
  caller_name?: string
  principal_id?: number
  credential_purpose_id?: number
  environment?: string
  rate_limit_enabled?: boolean
  rate_limit_window_seconds?: number
  rate_limit_max_requests?: number
  app_ids: number[]
}

/** Profile 更新请求体。后端为指针字段：省略 = 保持原值。禁止修改 token_id。 */
export interface UpdateIdentityProfilePayload {
  identity_mode?: IdentityMode
  attribution_target_type?: AttributionTargetType
  identity_assurance?: IdentityAssurance
  caller_id?: string
  caller_name?: string
  principal_id?: number
  credential_purpose_id?: number
  environment?: string
  rate_limit_enabled?: boolean
  rate_limit_window_seconds?: number
  rate_limit_max_requests?: number
  enabled?: boolean
}

/** App Binding 整体替换请求体（后端 replaceAppBindingsRequest）。 */
export interface ReplaceAppBindingsPayload {
  app_ids: number[]
}

// ---------------------------------------------------------------------------
// 列表 Query 类型（snake_case，显式映射，禁止依赖 Axios 自动转换）
// ---------------------------------------------------------------------------

export interface BusinessDomainListQuery extends GovernanceListQuery {}

export interface OwnerTeamListQuery extends GovernanceListQuery {}

export interface UsageTeamListQuery extends GovernanceListQuery {}

export interface PrincipalListQuery extends GovernanceListQuery {
  business_domain_id?: number
  usage_team_id?: number
}

export interface CredentialPurposeListQuery extends GovernanceListQuery {
  purpose_type?: CredentialPurposeType
}

export interface ApplicationListQuery extends GovernanceListQuery {
  business_domain_id?: number
  owner_team_id?: number
}

export interface IdentityProfileListQuery extends GovernanceListQuery {
  identity_mode?: IdentityMode
  token_id?: number
}

/** 身份审计真实查询参数（后端 GetIdentityAuditEvents）。 */
export interface IdentityAuditListQuery {
  page?: number
  page_size?: number
  request_id?: string
  token_id?: number
  profile_id?: number
  result?: IdentityAuditResult
  reason_code?: string
}

// ---------------------------------------------------------------------------
// 企业用量投影（§12 /api/ai-governance/usage/*）
// ---------------------------------------------------------------------------

export interface EnterpriseUsageRow {
  id: number
  bucket_time: number
  profile_id: number
  principal_id: number
  credential_purpose_id: number
  usage_business_domain_id: number
  usage_team_id: number
  caller_key: string
  root_app_code: string
  app_id: number
  app_business_domain_id: number
  owner_team_id: number
  identity_assurance: UsageIdentityAssurance
  client_verified: boolean
  model_name: string
  dimension_hash: string
  request_count: number
  success_count: number
  error_count: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  quota_net: number
  duration_ms_total: number
  created_at: number
  updated_at: number
}

/** 企业用量统计筛选（后端 parseUsageFilter，逐项对齐）。page/page_size 走服务端分页（E.2 P1-E）。 */
export interface EnterpriseUsageFilter {
  page?: number
  page_size?: number
  bucket_start?: number
  bucket_end?: number
  profile_id?: number
  principal_id?: number
  credential_purpose_id?: number
  usage_business_domain_id?: number
  usage_team_id?: number
  caller_key?: string
  root_app_code?: string
  app_business_domain_id?: number
  owner_team_id?: number
  identity_assurance?: UsageIdentityAssurance
  model_name?: string
}

export interface EnterpriseUsageAnomaly {
  profile_id: number
  principal_id: number
  credential_purpose_id: number
  bucket_time: number
  metric: 'request' | 'token' | 'quota'
  current: number
  baseline: number
  threshold: number
  model_name: string
  identity_assurance: UsageIdentityAssurance
}

export interface UsageRebuildResponse {
  processed_logs: number
}
