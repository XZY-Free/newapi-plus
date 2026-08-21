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
import { describe, expect, test } from 'vitest'

import type { GovernanceIdentityProfile } from '../../types'
import {
  buildIdentityProfileCreatePayload,
  buildIdentityProfileEditDelta,
  findIdentityProfileModeConfig,
  getIdentityProfileFormSchema,
  isLegalIdentityCombo,
  type IdentityProfileFormValues,
  type IdentityProfileModeConfig,
} from '../identity-profile-mode'

const t = ((key: string) => key) as TFunction

/** 冻结的四阶段组合必然合法，找不到即测试装置错误。 */
function configOf(
  mode: IdentityProfileModeConfig['identity_mode'],
  target: IdentityProfileModeConfig['attribution_target_type'],
  assurance: IdentityProfileModeConfig['identity_assurance']
): IdentityProfileModeConfig {
  const config = findIdentityProfileModeConfig(mode, target, assurance)
  if (!config) throw new Error(`expected legal combo: ${mode}/${target}/${assurance}`)
  return config
}

const CONFIG = {
  STATIC_PRINCIPAL: configOf('STATIC', 'PRINCIPAL', 'CREDENTIAL_ONLY'),
  STATIC_APPLICATION: configOf('STATIC', 'APPLICATION', 'CREDENTIAL_ONLY'),
  DYNAMIC_PLATFORM: configOf('DYNAMIC', 'PLATFORM', 'SIGNED_CONTEXT'),
  HYBRID_APPLICATION: configOf('HYBRID', 'APPLICATION', 'HYBRID_VERIFIED_CONTEXT'),
} satisfies Record<string, IdentityProfileModeConfig>

function formFor(
  config: IdentityProfileModeConfig,
  overrides: Partial<IdentityProfileFormValues> = {}
): IdentityProfileFormValues {
  return {
    token_id: 7,
    identity_mode: config.identity_mode,
    attribution_target_type: config.attribution_target_type,
    identity_assurance: config.identity_assurance,
    principal_id: null,
    credential_purpose_id: null,
    caller_id: '',
    caller_name: '',
    app_ids: [],
    environment: 'prod',
    rate_limit_enabled: false,
    rate_limit_window_seconds: '60',
    rate_limit_max_requests: '',
    ...overrides,
  }
}

describe('isLegalIdentityCombo', () => {
  test('exactly the four first-phase combinations are legal', () => {
    expect(
      isLegalIdentityCombo('STATIC', 'PRINCIPAL', 'CREDENTIAL_ONLY')
    ).toBe(true)
    expect(
      isLegalIdentityCombo('STATIC', 'APPLICATION', 'CREDENTIAL_ONLY')
    ).toBe(true)
    expect(
      isLegalIdentityCombo('DYNAMIC', 'PLATFORM', 'SIGNED_CONTEXT')
    ).toBe(true)
    expect(
      isLegalIdentityCombo('HYBRID', 'APPLICATION', 'HYBRID_VERIFIED_CONTEXT')
    ).toBe(true)
  })

  test('any fifth combination is rejected', () => {
    expect(
      isLegalIdentityCombo('STATIC', 'APPLICATION', 'SIGNED_CONTEXT')
    ).toBe(false)
    expect(
      isLegalIdentityCombo('DYNAMIC', 'PLATFORM', 'CREDENTIAL_ONLY')
    ).toBe(false)
    expect(
      isLegalIdentityCombo('HYBRID', 'PLATFORM', 'SIGNED_CONTEXT')
    ).toBe(false)
    expect(
      isLegalIdentityCombo('STATIC', 'PLATFORM', 'HYBRID_VERIFIED_CONTEXT')
    ).toBe(false)
  })
})

describe('buildIdentityProfileCreatePayload', () => {
  test('STATIC/PRINCIPAL keeps principal+purpose and clears caller and apps', () => {
    const payload = buildIdentityProfileCreatePayload(
      formFor(CONFIG.STATIC_PRINCIPAL, {
        principal_id: 5,
        credential_purpose_id: 4,
        caller_id: 'stale-caller',
        caller_name: 'stale',
        app_ids: [1, 2],
        rate_limit_enabled: true,
        rate_limit_window_seconds: '120',
        rate_limit_max_requests: '100',
      }),
      CONFIG.STATIC_PRINCIPAL
    )
    expect(payload).toEqual({
      token_id: 7,
      identity_mode: 'STATIC',
      attribution_target_type: 'PRINCIPAL',
      identity_assurance: 'CREDENTIAL_ONLY',
      caller_id: '',
      caller_name: '',
      principal_id: 5,
      credential_purpose_id: 4,
      environment: 'prod',
      rate_limit_enabled: true,
      rate_limit_window_seconds: 120,
      rate_limit_max_requests: 100,
      app_ids: [],
    })
    // Create Payload 绝不含 enabled（后端固定 enabled=false）。
    expect('enabled' in payload).toBe(false)
  })

  test('STATIC/APPLICATION clears principal, purpose and caller; app_ids exactly selected', () => {
    const payload = buildIdentityProfileCreatePayload(
      formFor(CONFIG.STATIC_APPLICATION, {
        principal_id: 5,
        credential_purpose_id: 4,
        caller_id: 'stale',
        app_ids: [3],
      }),
      CONFIG.STATIC_APPLICATION
    )
    expect(payload.principal_id).toBe(0)
    expect(payload.credential_purpose_id).toBe(0)
    expect(payload.caller_id).toBe('')
    expect(payload.caller_name).toBe('')
    expect(payload.app_ids).toEqual([3])
  })

  test('DYNAMIC/PLATFORM keeps caller and apps, clears principal and purpose', () => {
    const payload = buildIdentityProfileCreatePayload(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: '  platform-a ',
        caller_name: ' Platform A ',
        app_ids: [1, 2],
      }),
      CONFIG.DYNAMIC_PLATFORM
    )
    expect(payload.caller_id).toBe('platform-a')
    expect(payload.caller_name).toBe('Platform A')
    expect(payload.app_ids).toEqual([1, 2])
    expect(payload.principal_id).toBe(0)
    expect(payload.credential_purpose_id).toBe(0)
  })

  test('HYBRID/APPLICATION keeps caller, clears principal and purpose', () => {
    const payload = buildIdentityProfileCreatePayload(
      formFor(CONFIG.HYBRID_APPLICATION, {
        caller_id: 'bot',
        app_ids: [9],
      }),
      CONFIG.HYBRID_APPLICATION
    )
    expect(payload.caller_id).toBe('bot')
    expect(payload.app_ids).toEqual([9])
    expect(payload.principal_id).toBe(0)
    expect(payload.credential_purpose_id).toBe(0)
  })

  test('disabled rate limit sends window/max as 0', () => {
    const payload = buildIdentityProfileCreatePayload(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: 'x',
        app_ids: [1],
        rate_limit_enabled: false,
      }),
      CONFIG.DYNAMIC_PLATFORM
    )
    expect(payload.rate_limit_enabled).toBe(false)
    expect(payload.rate_limit_window_seconds).toBe(0)
    expect(payload.rate_limit_max_requests).toBe(0)
  })
})

describe('buildIdentityProfileEditDelta', () => {
  const current: GovernanceIdentityProfile = {
    id: 1,
    token_id: 7,
    identity_mode: 'STATIC',
    attribution_target_type: 'PRINCIPAL',
    identity_assurance: 'CREDENTIAL_ONLY',
    caller_id: 'old-caller',
    caller_name: 'Old',
    principal_id: 5,
    credential_purpose_id: 4,
    environment: 'prod',
    rate_limit_enabled: false,
    rate_limit_window_seconds: 0,
    rate_limit_max_requests: 0,
    enabled: true,
    created_at: 1,
    updated_at: 2,
  }

  test('STATIC/PRINCIPAL edit sends only changed fields', () => {
    const delta = buildIdentityProfileEditDelta(
      formFor(CONFIG.STATIC_PRINCIPAL, {
        principal_id: 5,
        credential_purpose_id: 4,
        environment: 'staging',
      }),
      current,
      CONFIG.STATIC_PRINCIPAL
    )
    expect(delta).toEqual({ environment: 'staging' })
    // 绝不重传核心三元组 / token_id / app_ids / enabled。
    const raw = delta as unknown as Record<string, unknown>
    expect(raw.identity_mode).toBeUndefined()
    expect(raw.attribution_target_type).toBeUndefined()
    expect(raw.identity_assurance).toBeUndefined()
    expect(raw.token_id).toBeUndefined()
    expect(raw.app_ids).toBeUndefined()
    expect(raw.enabled).toBeUndefined()
  })

  test('STATIC/PRINCIPAL edit sends principal/purpose when changed', () => {
    const delta = buildIdentityProfileEditDelta(
      formFor(CONFIG.STATIC_PRINCIPAL, {
        principal_id: 99,
        credential_purpose_id: 88,
      }),
      current,
      CONFIG.STATIC_PRINCIPAL
    )
    expect(delta.principal_id).toBe(99)
    expect(delta.credential_purpose_id).toBe(88)
  })

  test('enabling rate limit sends window and max', () => {
    const delta = buildIdentityProfileEditDelta(
      formFor(CONFIG.STATIC_PRINCIPAL, {
        principal_id: 5,
        credential_purpose_id: 4,
        rate_limit_enabled: true,
        rate_limit_window_seconds: '120',
        rate_limit_max_requests: '1000',
      }),
      current,
      CONFIG.STATIC_PRINCIPAL
    )
    expect(delta.rate_limit_enabled).toBe(true)
    expect(delta.rate_limit_window_seconds).toBe(120)
    expect(delta.rate_limit_max_requests).toBe(1000)
  })

  test('no change yields an empty delta', () => {
    const delta = buildIdentityProfileEditDelta(
      formFor(CONFIG.STATIC_PRINCIPAL, {
        principal_id: 5,
        credential_purpose_id: 4,
        environment: 'prod',
      }),
      current,
      CONFIG.STATIC_PRINCIPAL
    )
    expect(delta).toEqual({})
  })
})

describe('getIdentityProfileFormSchema', () => {
  test('STATIC/PRINCIPAL requires principal and purpose', () => {
    const schema = getIdentityProfileFormSchema(t)
    const ok = schema.safeParse(
      formFor(CONFIG.STATIC_PRINCIPAL, { principal_id: 5, credential_purpose_id: 4 })
    )
    expect(ok.success).toBe(true)

    const missing = schema.safeParse(formFor(CONFIG.STATIC_PRINCIPAL))
    expect(missing.success).toBe(false)
  })

  test('STATIC/APPLICATION requires exactly one app', () => {
    const schema = getIdentityProfileFormSchema(t)
    expect(
      schema.safeParse(formFor(CONFIG.STATIC_APPLICATION, { app_ids: [1] })).success
    ).toBe(true)
    expect(
      schema.safeParse(formFor(CONFIG.STATIC_APPLICATION, { app_ids: [1, 2] })).success
    ).toBe(false)
    expect(
      schema.safeParse(formFor(CONFIG.STATIC_APPLICATION, { app_ids: [] })).success
    ).toBe(false)
  })

  test('DYNAMIC/PLATFORM requires at least one app and a caller_id', () => {
    const schema = getIdentityProfileFormSchema(t)
    expect(
      schema.safeParse(
        formFor(CONFIG.DYNAMIC_PLATFORM, { caller_id: 'x', app_ids: [1] })
      ).success
    ).toBe(true)
    const noApp = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, { caller_id: 'x', app_ids: [] })
    )
    expect(noApp.success).toBe(false)
    const noCaller = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, { caller_id: '', app_ids: [1] })
    )
    expect(noCaller.success).toBe(false)
  })

  test('fifth (illegal) combination is rejected by the schema', () => {
    const schema = getIdentityProfileFormSchema(t)
    const fifth = formFor(CONFIG.DYNAMIC_PLATFORM, {
      identity_assurance: 'CREDENTIAL_ONLY',
      caller_id: 'x',
      app_ids: [1],
    })
    expect(schema.safeParse(fifth).success).toBe(false)
  })

  test('rate limit boundaries: 10/3600 and 1/100000 accepted, out-of-range rejected', () => {
    const schema = getIdentityProfileFormSchema(t)
    const base = () =>
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: 'x',
        app_ids: [1],
        rate_limit_enabled: true,
      })

    expect(
      schema.safeParse({
        ...base(),
        rate_limit_window_seconds: '10',
        rate_limit_max_requests: '1',
      }).success
    ).toBe(true)
    expect(
      schema.safeParse({
        ...base(),
        rate_limit_window_seconds: '3600',
        rate_limit_max_requests: '100000',
      }).success
    ).toBe(true)

    const lowWindow = schema.safeParse({
      ...base(),
      rate_limit_window_seconds: '9',
      rate_limit_max_requests: '100',
    })
    expect(lowWindow.success).toBe(false)
    const highWindow = schema.safeParse({
      ...base(),
      rate_limit_window_seconds: '3601',
      rate_limit_max_requests: '100',
    })
    expect(highWindow.success).toBe(false)
    const lowMax = schema.safeParse({
      ...base(),
      rate_limit_window_seconds: '60',
      rate_limit_max_requests: '0',
    })
    expect(lowMax.success).toBe(false)
    const highMax = schema.safeParse({
      ...base(),
      rate_limit_window_seconds: '60',
      rate_limit_max_requests: '100001',
    })
    expect(highMax.success).toBe(false)
  })

  test('disabled rate limit ignores window/max (empty is valid)', () => {
    const schema = getIdentityProfileFormSchema(t)
    expect(
      schema.safeParse(
        formFor(CONFIG.DYNAMIC_PLATFORM, {
          caller_id: 'x',
          app_ids: [1],
          rate_limit_enabled: false,
          rate_limit_window_seconds: '',
          rate_limit_max_requests: '',
        })
      ).success
    ).toBe(true)
  })

  test('caller and environment length limits', () => {
    const schema = getIdentityProfileFormSchema(t)
    const longCaller = 'x'.repeat(129)
    const callerOk = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: 'x'.repeat(128),
        app_ids: [1],
      })
    )
    expect(callerOk.success).toBe(true)
    const callerTooLong = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: longCaller,
        app_ids: [1],
      })
    )
    expect(callerTooLong.success).toBe(false)

    const envOk = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: 'x',
        app_ids: [1],
        environment: 'x'.repeat(32),
      })
    )
    expect(envOk.success).toBe(true)
    const envTooLong = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: 'x',
        app_ids: [1],
        environment: 'x'.repeat(33),
      })
    )
    expect(envTooLong.success).toBe(false)
    const envEmpty = schema.safeParse(
      formFor(CONFIG.DYNAMIC_PLATFORM, {
        caller_id: 'x',
        app_ids: [1],
        environment: ' ',
      })
    )
    expect(envEmpty.success).toBe(false)
  })
})
