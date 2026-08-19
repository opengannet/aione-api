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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getChannelActionPermissions } from './channel-action-permissions'

describe('channel action permissions', () => {
  test('allows channel.write to open managed tuning without definition actions', () => {
    assert.deepEqual(
      getChannelActionPermissions({
        flyte2Managed: true,
        canWrite: true,
        canSensitiveWrite: false,
      }),
      { canEdit: true, canMutateDefinition: false }
    )
  })

  test('keeps sensitive_write as the requirement for ordinary channels', () => {
    assert.deepEqual(
      getChannelActionPermissions({
        flyte2Managed: false,
        canWrite: true,
        canSensitiveWrite: false,
      }),
      { canEdit: false, canMutateDefinition: false }
    )
    assert.deepEqual(
      getChannelActionPermissions({
        flyte2Managed: false,
        canWrite: false,
        canSensitiveWrite: true,
      }),
      { canEdit: true, canMutateDefinition: true }
    )
  })

  test('keeps copy and delete definition actions disabled for managed channels', () => {
    assert.deepEqual(
      getChannelActionPermissions({
        flyte2Managed: true,
        canWrite: true,
        canSensitiveWrite: true,
      }),
      { canEdit: true, canMutateDefinition: false }
    )
  })
})
