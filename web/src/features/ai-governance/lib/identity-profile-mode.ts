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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import type {
  AttributionTargetType,
  CreateIdentityProfilePayload,
  GovernanceIdentityProfile,
  IdentityAssurance,
  IdentityMode,
  UpdateIdentityProfilePayload,
} from '../types'

/**
 * Identity Profile 身份模式**唯一事实源**（§11-C §C.2）。
 *
 * 第一阶段只允许四种合法组合（模式/归因目标/可信等级联动），禁止任何 JSX 里
 * 到处散落 `if STATIC...` 分支。所有需要“该模式用什么字段、app 数量、合法组合”
 * 的地方都必须从 {@link IDENTITY_PROFILE_MODES} 派生。
 *
 * | identity_mode | attribution_target_type | identity_assurance  | principal/purpose | caller | app_ids |
 * |---|---|---|---|---|---|
 * | STATIC   | PRINCIPAL   | CREDENTIAL_ONLY         | 必填           | 清空   | []（恰好 0） |
 * | STATIC   | APPLICATION | CREDENTIAL_ONLY         | 清空           | 清空   | 恰好 1       |
 * | DYNAMIC  | PLATFORM    | SIGNED_CONTEXT          | 清空           | 必填   | ≥ 1          |
 * | HYBRID   | APPLICATION | HYBRID_VERIFIED_CONTEXT | 清空           | 必填   | 恰好 1       |
 *
 * `identity_assurance` 不是自由枚举：它由合法组合约束，管理员不能配出第五种组合。
 */

export interface IdentityProfileModeConfig {
  identity_mode: IdentityMode
  attribution_target_type: AttributionTargetType
  identity_assurance: IdentityAssurance
  /** 归因到使用主体（STATIC/PRINCIPAL 专用）。 */
  usesPrincipal: boolean
  /** 凭证用途（STATIC/PRINCIPAL 专用）。 */
  usesPurpose: boolean
  /** Caller（DYNAMIC/PLATFORM 与 HYBRID/APPLICATION 使用）。 */
  usesCaller: boolean
  /** 是否展示并绑定应用（STATIC/PRINCIPAL 不绑定，app_ids=[]）。 */
  usesApps: boolean
  /** 合法 App 数量下限。 */
  appMin: number
  /** 合法 App 数量上限（Infinity 表示无上限）。 */
  appMax: number
}

export const IDENTITY_PROFILE_MODES: readonly IdentityProfileModeConfig[] = [
  {
    identity_mode: 'STATIC',
    attribution_target_type: 'PRINCIPAL',
    identity_assurance: 'CREDENTIAL_ONLY',
    usesPrincipal: true,
    usesPurpose: true,
    usesCaller: false,
    usesApps: false,
    appMin: 0,
    appMax: 0,
  },
  {
    identity_mode: 'STATIC',
    attribution_target_type: 'APPLICATION',
    identity_assurance: 'CREDENTIAL_ONLY',
    usesPrincipal: false,
    usesPurpose: false,
    usesCaller: false,
    usesApps: true,
    appMin: 1,
    appMax: 1,
  },
  {
    identity_mode: 'DYNAMIC',
    attribution_target_type: 'PLATFORM',
    identity_assurance: 'SIGNED_CONTEXT',
    usesPrincipal: false,
    usesPurpose: false,
    usesCaller: true,
    usesApps: true,
    appMin: 1,
    appMax: Number.POSITIVE_INFINITY,
  },
  {
    identity_mode: 'HYBRID',
    attribution_target_type: 'APPLICATION',
    identity_assurance: 'HYBRID_VERIFIED_CONTEXT',
    usesPrincipal: false,
    usesPurpose: false,
    usesCaller: true,
    usesApps: true,
    appMin: 1,
    appMax: 1,
  },
]

/** 该 (mode, target, assurance) 是否为四种合法组合之一。 */
export function isLegalIdentityCombo(
  identityMode: IdentityMode,
  attributionTargetType: AttributionTargetType,
  identityAssurance: IdentityAssurance
): boolean {
  return IDENTITY_PROFILE_MODES.some(
    (c) =>
      c.identity_mode === identityMode &&
      c.attribution_target_type === attributionTargetType &&
      c.identity_assurance === identityAssurance
  )
}

/** 由三元组解析模式配置；非法组合返回 undefined。 */
export function findIdentityProfileModeConfig(
  identityMode: IdentityMode,
  attributionTargetType: AttributionTargetType,
  identityAssurance: IdentityAssurance
): IdentityProfileModeConfig | undefined {
  return IDENTITY_PROFILE_MODES.find(
    (c) =>
      c.identity_mode === identityMode &&
      c.attribution_target_type === attributionTargetType &&
      c.identity_assurance === identityAssurance
  )
}

// ---------------------------------------------------------------------------
// 字段长度 / 限流边界（后端作为最终边界，前端预校验一致）
// ---------------------------------------------------------------------------

export const CALLER_MAX_LENGTH = 128
export const ENVIRONMENT_MAX_LENGTH = 32
export const ENVIRONMENT_DEFAULT = 'prod'

export const RATE_LIMIT_WINDOW_MIN = 10
export const RATE_LIMIT_WINDOW_MAX = 3600
export const RATE_LIMIT_WINDOW_DEFAULT = 60
export const RATE_LIMIT_MAX_REQUESTS_MIN = 1
export const RATE_LIMIT_MAX_REQUESTS_MAX = 100000

// ---------------------------------------------------------------------------
// 表单值模型（RHF + zod 的单一事实源）
// ---------------------------------------------------------------------------

export type IdentityProfileFormValues = {
  identity_mode: IdentityMode
  attribution_target_type: AttributionTargetType
  identity_assurance: IdentityAssurance
  /** Create 必填；Edit 只读（创建后永久不可改）。 */
  token_id: number | null
  principal_id: number | null
  credential_purpose_id: number | null
  caller_id: string
  caller_name: string
  /** 已选应用 DB id 集合（App Bindings 仅 Create 设置，普通 Edit 不触碰）。 */
  app_ids: number[]
  environment: string
  rate_limit_enabled: boolean
  /** 以字符串承载，便于区分「空串未填」与「合法 0」。 */
  rate_limit_window_seconds: string
  rate_limit_max_requests: string
}

export function identityProfileFormDefaults(): IdentityProfileFormValues {
  return {
    identity_mode: 'STATIC',
    attribution_target_type: 'PRINCIPAL',
    identity_assurance: 'CREDENTIAL_ONLY',
    token_id: null,
    principal_id: null,
    credential_purpose_id: null,
    caller_id: '',
    caller_name: '',
    app_ids: [],
    environment: ENVIRONMENT_DEFAULT,
    rate_limit_enabled: false,
    rate_limit_window_seconds: String(RATE_LIMIT_WINDOW_DEFAULT),
    rate_limit_max_requests: '',
  }
}

/**
 * 由聚合详情派生「重配置身份模式」默认值（§11-C §C.3 E）。
 *
 * 重配置从当前模式起步（字段沿用），但允许切换核心三元组；`app_ids` 预填当前
 * 已绑定应用，供管理员在绑定优先的流程中先调整 App Binding 再写 Profile。
 */
export function identityProfileReconfigureDefaults(
  detail: { profile: GovernanceIdentityProfile; bindings: { app_id: number }[] }
): IdentityProfileFormValues {
  return {
    ...identityProfileEditDefaults(detail.profile),
    app_ids: detail.bindings.map((b) => b.app_id),
  }
}

/** 由既有 Profile 行派生 Edit 表单默认值（核心三元组与 token_id 只读沿用）。 */
export function identityProfileEditDefaults(
  profile: GovernanceIdentityProfile
): IdentityProfileFormValues {
  const base = identityProfileFormDefaults()
  return {
    ...base,
    identity_mode: profile.identity_mode,
    attribution_target_type: profile.attribution_target_type,
    identity_assurance: profile.identity_assurance,
    token_id: profile.token_id,
    principal_id: profile.principal_id > 0 ? profile.principal_id : null,
    credential_purpose_id:
      profile.credential_purpose_id > 0 ? profile.credential_purpose_id : null,
    caller_id: profile.caller_id,
    caller_name: profile.caller_name,
    environment: profile.environment,
    rate_limit_enabled: profile.rate_limit_enabled,
    rate_limit_window_seconds: String(
      profile.rate_limit_window_seconds || RATE_LIMIT_WINDOW_DEFAULT
    ),
    rate_limit_max_requests: String(
      profile.rate_limit_max_requests || ''
    ),
  }
}

// ---------------------------------------------------------------------------
// 校验 Schema（Create / Edit 共用同一份，含按模式的必填与清理语义）
// ---------------------------------------------------------------------------

/** RuneCount（与后端 Unicode 长度校验一致）。 */
function runeCount(value: string): number {
  return [...value].length
}

/**
 * Create 与普通 Edit 共用同一份校验 Schema（单一事实源）。
 * `opts.edit=true` 时跳过 App 绑定数量校验——普通 Edit（C.2）不负责修改现有
 * App Bindings（那属于 C.3），app_ids 恒为 []，不应因未绑定而拒绝保存。
 */
export function getIdentityProfileFormSchema(
  t: TFunction,
  opts: { edit?: boolean } = {}
) {
  return z
    .object({
      identity_mode: z.enum(['STATIC', 'DYNAMIC', 'HYBRID']),
      attribution_target_type: z.enum(['PRINCIPAL', 'APPLICATION', 'PLATFORM']),
      identity_assurance: z.enum([
        'CREDENTIAL_ONLY',
        'SIGNED_CONTEXT',
        'HYBRID_VERIFIED_CONTEXT',
      ]),
      token_id: z.number().nullable(),
      principal_id: z.number().nullable(),
      credential_purpose_id: z.number().nullable(),
      caller_id: z.string(),
      caller_name: z.string(),
      app_ids: z.array(z.number()),
      environment: z.string(),
      rate_limit_enabled: z.boolean(),
      rate_limit_window_seconds: z.string(),
      rate_limit_max_requests: z.string(),
    })
    .superRefine((data, ctx) => {
      const config = findIdentityProfileModeConfig(
        data.identity_mode,
        data.attribution_target_type,
        data.identity_assurance
      )
      if (!config) {
        ctx.addIssue({
          code: 'custom',
          path: ['identity_mode'],
          message: t('Select a valid identity configuration'),
        })
        return
      }

      if (data.token_id == null) {
        ctx.addIssue({
          code: 'custom',
          path: ['token_id'],
          message: t('Please select an API key'),
        })
      }

      if (config.usesPrincipal && data.principal_id == null) {
        ctx.addIssue({
          code: 'custom',
          path: ['principal_id'],
          message: t('Please select a principal'),
        })
      }
      if (config.usesPurpose && data.credential_purpose_id == null) {
        ctx.addIssue({
          code: 'custom',
          path: ['credential_purpose_id'],
          message: t('Please select a credential purpose'),
        })
      }

      if (config.usesCaller) {
        const callerId = data.caller_id.trim()
        if (!callerId) {
          ctx.addIssue({
            code: 'custom',
            path: ['caller_id'],
            message: t('Caller ID is required'),
          })
        }
        if (runeCount(callerId) > CALLER_MAX_LENGTH) {
          ctx.addIssue({
            code: 'custom',
            path: ['caller_id'],
            message: t('Caller ID must be at most {{n}} characters').replace(
              '{{n}}',
              String(CALLER_MAX_LENGTH)
            ),
          })
        }
        if (runeCount(data.caller_name.trim()) > CALLER_MAX_LENGTH) {
          ctx.addIssue({
            code: 'custom',
            path: ['caller_name'],
            message: t('Caller name must be at most {{n}} characters').replace(
              '{{n}}',
              String(CALLER_MAX_LENGTH)
            ),
          })
        }
      }

      const environment = data.environment.trim()
      if (!environment) {
        ctx.addIssue({
          code: 'custom',
          path: ['environment'],
          message: t('Environment is required'),
        })
      } else if (runeCount(environment) > ENVIRONMENT_MAX_LENGTH) {
        ctx.addIssue({
          code: 'custom',
          path: ['environment'],
          message: t('Environment must be at most {{n}} characters').replace(
            '{{n}}',
            String(ENVIRONMENT_MAX_LENGTH)
          ),
        })
      }

      if (config.usesApps && !opts.edit) {
        const count = data.app_ids.length
        if (count < config.appMin) {
          ctx.addIssue({
            code: 'custom',
            path: ['app_ids'],
            message:
              config.appMin === config.appMax
                ? t('Select exactly {{n}} application', { n: config.appMin })
                : t('Select at least {{n}} application', { n: config.appMin }),
          })
        }
        if (config.appMax !== Number.POSITIVE_INFINITY && count > config.appMax) {
          ctx.addIssue({
            code: 'custom',
            path: ['app_ids'],
            message: t('Select at most {{n}} application', {
              n: config.appMax,
            }),
          })
        }
      }

      if (data.rate_limit_enabled) {
        const windowText = data.rate_limit_window_seconds.trim()
        const windowValue = Number(windowText)
        const windowOk =
          /^\d+$/.test(windowText) &&
          windowValue >= RATE_LIMIT_WINDOW_MIN &&
          windowValue <= RATE_LIMIT_WINDOW_MAX
        if (!windowOk) {
          ctx.addIssue({
            code: 'custom',
            path: ['rate_limit_window_seconds'],
            message: t(
              'Rate limit window must be between {{min}} and {{max}} seconds'
            )
              .replace('{{min}}', String(RATE_LIMIT_WINDOW_MIN))
              .replace('{{max}}', String(RATE_LIMIT_WINDOW_MAX)),
          })
        }
        const maxText = data.rate_limit_max_requests.trim()
        const maxValue = Number(maxText)
        const maxOk =
          /^\d+$/.test(maxText) &&
          maxValue >= RATE_LIMIT_MAX_REQUESTS_MIN &&
          maxValue <= RATE_LIMIT_MAX_REQUESTS_MAX
        if (!maxOk) {
          ctx.addIssue({
            code: 'custom',
            path: ['rate_limit_max_requests'],
            message: t(
              'Rate limit max requests must be between {{min}} and {{max}}'
            )
              .replace('{{min}}', String(RATE_LIMIT_MAX_REQUESTS_MIN))
              .replace('{{max}}', String(RATE_LIMIT_MAX_REQUESTS_MAX)),
          })
        }
      }
    })
}

// ---------------------------------------------------------------------------
// Payload 构建（Create / Edit 共用同一套规范化）
// ---------------------------------------------------------------------------

/**
 * 依据目标模式把表单值规范化为 Create 请求体。
 *
 * 硬要求：隐藏字段必须被清理，禁止残留“看起来是 DYNAMIC、数据库还挂着
 * Principal”。STATIC 清 Caller；PRINCIPAL 清 App Bindings；APPLICATION/PLATFORM
 * 清 Principal/Purpose。Create Payload 绝不含 `enabled`（后端固定 enabled=false）。
 */
export function buildIdentityProfileCreatePayload(
  values: IdentityProfileFormValues,
  config: IdentityProfileModeConfig
): CreateIdentityProfilePayload {
  return {
    token_id: values.token_id ?? 0,
    identity_mode: values.identity_mode,
    attribution_target_type: values.attribution_target_type,
    identity_assurance: values.identity_assurance,
    caller_id: config.usesCaller ? values.caller_id.trim() : '',
    caller_name: config.usesCaller ? values.caller_name.trim() : '',
    principal_id: config.usesPrincipal ? values.principal_id ?? 0 : 0,
    credential_purpose_id: config.usesPurpose
      ? values.credential_purpose_id ?? 0
      : 0,
    environment: values.environment.trim(),
    rate_limit_enabled: values.rate_limit_enabled,
    rate_limit_window_seconds: values.rate_limit_enabled
      ? Number(values.rate_limit_window_seconds) || 0
      : 0,
    rate_limit_max_requests: values.rate_limit_enabled
      ? Number(values.rate_limit_max_requests) || 0
      : 0,
    app_ids: config.usesApps ? values.app_ids : [],
  }
}

/**
 * 当前身份模式内部编辑的 Delta Payload（§11-C §C.2 十二）。
 *
 * 普通 Edit 不负责核心三元组（identity_mode / attribution_target_type /
 * identity_assurance）、不负责 token_id、不负责 App Bindings——那些留给 C.3。
 * 只发送实际变化字段，绝不整份 PUT 回去，也绝不重传未变的核心三元组；
 * `enabled` 继续由列表启停行操作管理。
 */
export function buildIdentityProfileEditDelta(
  values: IdentityProfileFormValues,
  current: GovernanceIdentityProfile,
  config: IdentityProfileModeConfig
): UpdateIdentityProfilePayload {
  const delta: UpdateIdentityProfilePayload = {}

  const environment = values.environment.trim()
  if (environment !== current.environment) delta.environment = environment

  if (values.rate_limit_enabled !== current.rate_limit_enabled) {
    delta.rate_limit_enabled = values.rate_limit_enabled
  }
  if (values.rate_limit_enabled) {
    const windowValue = Number(values.rate_limit_window_seconds) || 0
    const maxValue = Number(values.rate_limit_max_requests) || 0
    if (windowValue !== current.rate_limit_window_seconds) {
      delta.rate_limit_window_seconds = windowValue
    }
    if (maxValue !== current.rate_limit_max_requests) {
      delta.rate_limit_max_requests = maxValue
    }
  }

  if (config.usesPrincipal && values.principal_id != null) {
    if (values.principal_id !== current.principal_id) {
      delta.principal_id = values.principal_id
    }
  }
  if (config.usesPurpose && values.credential_purpose_id != null) {
    if (values.credential_purpose_id !== current.credential_purpose_id) {
      delta.credential_purpose_id = values.credential_purpose_id
    }
  }

  if (config.usesCaller) {
    const callerId = values.caller_id.trim()
    const callerName = values.caller_name.trim()
    if (callerId !== current.caller_id) delta.caller_id = callerId
    if (callerName !== current.caller_name) delta.caller_name = callerName
  }

  return delta
}

/**
 * 重配置身份模式（§11-C §C.3 E）的分步计划：目标 App Binding 集合 + 目标 Profile Patch。
 *
 * 与普通 Edit 的 Delta 不同，重配置会切换核心三元组，因此必须**显式地设置/清理**
 * 每个模式的隐藏字段（STATIC 清 Caller；APPLICATION/PLATFORM 清 Principal/Purpose），
 * 绝不允许残留旧模式的隐藏字段。
 *
 * 执行顺序固定为「先 Replace App Bindings，再 Update Profile」，由页面层编排；
 * 本函数只负责生成两段规范化后的请求体，绝不直接调用后端。
 */
export function buildIdentityProfileReconfigurePlan(
  values: IdentityProfileFormValues,
  config: IdentityProfileModeConfig
): { app_ids: number[]; profilePatch: UpdateIdentityProfilePayload } {
  const profilePatch: UpdateIdentityProfilePayload = {
    identity_mode: values.identity_mode,
    attribution_target_type: values.attribution_target_type,
    identity_assurance: values.identity_assurance,
    caller_id: config.usesCaller ? values.caller_id.trim() : '',
    caller_name: config.usesCaller ? values.caller_name.trim() : '',
    principal_id: config.usesPrincipal ? values.principal_id ?? 0 : 0,
    credential_purpose_id: config.usesPurpose
      ? values.credential_purpose_id ?? 0
      : 0,
  }
  return {
    app_ids: config.usesApps ? values.app_ids : [],
    profilePatch,
  }
}
