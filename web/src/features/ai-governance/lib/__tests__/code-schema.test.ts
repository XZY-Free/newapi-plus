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
import { z } from 'zod'

import {
  getDomainCodeSchema,
  getNameSchema,
  getSimpleCodeSchema,
  PRINCIPAL_CODE_MAX_LENGTH,
  SIMPLE_CODE_MAX_LENGTH,
} from '../code-schema'
import { purposeTypeSchema } from '../credential-purpose-types'

/** 校验测试只需要 message key；空 i18n 下 t(key) 返回 key 本身即可。 */
const t = ((key: string) => key) as unknown as TFunction

describe('code-schema：与后端 service/ai_identity.go 校验一致（§11-B.1 P1-6）', () => {
  describe('getDomainCodeSchema（domain_code / app_code：小写开头、[a-z0-9._-]、2~64）', () => {
    const schema = getDomainCodeSchema(t)
    test('接受合法的小写标识符', () => {
      for (const code of ['hr', 'hr-eng', 'a1', 'a.b_c-d', 'x'.repeat(64)]) {
        expect(schema.safeParse(code).success).toBe(true)
      }
    })
    test('拒绝大写开头 / 数字开头 / 空格', () => {
      for (const code of ['Hr', '1hr', 'hr eng', 'h']) {
        expect(schema.safeParse(code).success).toBe(false)
      }
    })
    test('拒绝超过 64 字符', () => {
      expect(schema.safeParse('x'.repeat(65)).success).toBe(false)
    })
  })

  describe('getSimpleCodeSchema（team / purpose code：非空、无空白、≤64）', () => {
    const schema = getSimpleCodeSchema(t, SIMPLE_CODE_MAX_LENGTH)
    test('接受非空无空白 code（含数字/点/横线）', () => {
      for (const code of ['abc', 'a-b_c.d', '123', 'x'.repeat(64)]) {
        expect(schema.safeParse(code).success).toBe(true)
      }
    })
    test('拒绝空串 / 含空白', () => {
      expect(schema.safeParse('').success).toBe(false)
      for (const code of ['ab cd', 'a\tb', 'a\nb', ' ab']) {
        expect(schema.safeParse(code).success).toBe(false)
      }
    })
    test('拒绝超过 64 字符', () => {
      expect(schema.safeParse('x'.repeat(65)).success).toBe(false)
    })
  })

  describe('getSimpleCodeSchema 的 principal 变体（≤128）', () => {
    const schema = getSimpleCodeSchema(t, PRINCIPAL_CODE_MAX_LENGTH)
    test('接受 128 字符，拒绝 129', () => {
      expect(schema.safeParse('x'.repeat(128)).success).toBe(true)
      expect(schema.safeParse('x'.repeat(129)).success).toBe(false)
    })
  })

  describe('getNameSchema（RuneCount ≤ 128，对应后端 validateNameLen）', () => {
    const schema = getNameSchema(t)
    test('接受非空名称（含 128 字符）', () => {
      expect(schema.safeParse('a').success).toBe(true)
      expect(schema.safeParse('x'.repeat(128)).success).toBe(true)
    })
    test('拒绝空名称 / 超过 128 字符', () => {
      expect(schema.safeParse('').success).toBe(false)
      expect(schema.safeParse('x'.repeat(129)).success).toBe(false)
    })
  })
})

describe('purposeTypeSchema（§11-B.1 P2-1：正式枚举，拒绝非法值）', () => {
  test('接受全部五个枚举原值', () => {
    for (const v of [
      'DESKTOP_CLIENT',
      'IDE',
      'SCRIPT',
      'SERVICE',
      'OTHER',
    ]) {
      expect(purposeTypeSchema.safeParse(v).success).toBe(true)
    }
  })
  test('拒绝大小写错误 / 未知枚举 / 空串', () => {
    for (const v of ['Service', 'desktop_client', 'BOGUS', '']) {
      expect(purposeTypeSchema.safeParse(v).success).toBe(false)
    }
  })
  test('表单「未选择」态由 or(z.literal("")) 表达，本身不属于枚举', () => {
    expect(purposeTypeSchema.safeParse('').success).toBe(false)
    const formSchema = purposeTypeSchema.or(z.literal(''))
    expect(formSchema.safeParse('').success).toBe(true)
  })
})
