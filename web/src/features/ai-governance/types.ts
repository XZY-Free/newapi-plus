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
 * 企业 AI 治理公共领域模型（V1.1 §11）。
 *
 * 本文件是对后端 `/api/ai-governance/*` 返回结构的唯一事实来源，作为前端类型边界。
 * 字段命名与后端 model 的 JSON tag 一一对应（业务领域/使用团队/使用主体/凭证用途/
 * 应用团队/AI 应用/API Key 身份/签名密钥/身份审计/企业用量投影）。
 */

// ---------------------------------------------------------------------------
// 通用响应包装
// ---------------------------------------------------------------------------

export interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}

// ---------------------------------------------------------------------------
// 主数据
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
  principal_type: string
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
  purpose_type: string
  enabled: boolean
  created_at: number
  updated_at: number
}

// ---------------------------------------------------------------------------
// AI 应用
// ---------------------------------------------------------------------------

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
// API Key 身份
// ---------------------------------------------------------------------------

export type IdentityMode = 'STATIC' | 'DYNAMIC' | 'HYBRID'
export type AttributionTargetType =
  | 'PRINCIPAL'
  | 'APPLICATION'
  | 'PLATFORM'
export type IdentityAssurance =
  | 'CREDENTIAL_ONLY'
  | 'SIGNED_CONTEXT'
  | 'HYBRID_VERIFIED_CONTEXT'

export interface GovernanceIdentityProfile {
  id: number
  token_id: number
  identity_mode: string
  attribution_target_type: string
  identity_assurance: string
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

export interface GovernanceIdentityAppBinding {
  id: number
  profile_id: number
  app_id: number
  enabled: boolean
  created_at: number
  updated_at: number
}

// SigningKey 列表只返回元数据，绝不包含明文/密文。
export interface GovernanceSigningKey {
  id: number
  profile_id: number
  key_id: string
  status: string
  not_before: number
  expires_at: number
  revoked_at: number
  created_at: number
  updated_at: number
}

// ---------------------------------------------------------------------------
// 身份审计
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
  identity_mode: string
  identity_assurance: string
  result: string
  reason_code: string
  claimed_root_app_id: string
  http_method: string
  request_path: string
  client_ip: string
}

// ---------------------------------------------------------------------------
// 企业用量投影（对接 §12 /api/ai-governance/usage/*）
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
  identity_assurance: string
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

export interface EnterpriseUsageFilter {
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
  identity_assurance?: string
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
  identity_assurance: string
}

export interface UsageRebuildResponse {
  processed_logs: number
}
