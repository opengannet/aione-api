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
import { describe, expect, test } from 'vitest'

import { getChannelActionPermissions } from './channel-action-permissions'

describe('channel action permissions', () => {
  test('allows channel.write to open managed tuning without definition actions', () => {
    expect(
      getChannelActionPermissions({
        flyte2Managed: true,
        canWrite: true,
        canSensitiveWrite: false,
      })
    ).toEqual({ canEdit: true, canMutateDefinition: false })
  })

  test('keeps sensitive_write as the requirement for ordinary channels', () => {
    expect(
      getChannelActionPermissions({
        flyte2Managed: false,
        canWrite: true,
        canSensitiveWrite: false,
      })
    ).toEqual({ canEdit: false, canMutateDefinition: false })
    expect(
      getChannelActionPermissions({
        flyte2Managed: false,
        canWrite: false,
        canSensitiveWrite: true,
      })
    ).toEqual({ canEdit: true, canMutateDefinition: true })
  })

  test('keeps copy and delete definition actions disabled for managed channels', () => {
    expect(
      getChannelActionPermissions({
        flyte2Managed: true,
        canWrite: true,
        canSensitiveWrite: true,
      })
    ).toEqual({ canEdit: true, canMutateDefinition: false })
  })
})
