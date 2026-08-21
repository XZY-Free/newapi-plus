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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { listIdentityProfiles } from '../../api'
import { hasExistingProfileForToken } from '../token-profile-exists'

vi.mock('../../api', () => ({
  listIdentityProfiles: vi.fn(),
}))

describe('hasExistingProfileForToken', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  test('probes listIdentityProfiles with token_id page 1 size 1 and returns true when bound', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [],
      total: 1,
      page: 1,
      page_size: 1,
    })
    await expect(hasExistingProfileForToken(7)).resolves.toBe(true)
    expect(listIdentityProfiles).toHaveBeenCalledWith({
      token_id: 7,
      page: 1,
      page_size: 1,
    })
  })

  test('returns false when the token has no profile yet', async () => {
    vi.mocked(listIdentityProfiles).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 1,
    })
    await expect(hasExistingProfileForToken(7)).resolves.toBe(false)
  })

  test('returns false without issuing a request for an invalid token id', async () => {
    await expect(hasExistingProfileForToken(0)).resolves.toBe(false)
    await expect(hasExistingProfileForToken(-3)).resolves.toBe(false)
    expect(listIdentityProfiles).not.toHaveBeenCalled()
  })
})
