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

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformFormDataToFlyteManagedTuningPayload,
} from './channel-form'

describe('Flyte2 managed channel tuning payload', () => {
  test('only includes the four fields allowed by the managed update API', () => {
    const payload = transformFormDataToFlyteManagedTuningPayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'must-not-be-sent',
        models: 'gpt-4.1,claude-sonnet-4',
        base_url: 'https://example.com',
        key: 'secret',
        priority: 12,
        weight: 34,
        tag: 'production',
      },
      42
    )

    expect(payload).toEqual({
      id: 42,
      priority: 12,
      weight: 34,
      tag: 'production',
    })
    expect(Object.keys(payload).sort()).toEqual([
      'id',
      'priority',
      'tag',
      'weight',
    ])
  })

  test('preserves zero priority, zero weight, and an empty tag', () => {
    const payload = transformFormDataToFlyteManagedTuningPayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        priority: 0,
        weight: 0,
        tag: '',
      },
      7
    )

    expect(payload).toEqual({
      id: 7,
      priority: 0,
      weight: 0,
      tag: '',
    })
  })
})
