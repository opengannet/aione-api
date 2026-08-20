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

import {
  createDeployment,
  deleteDeployment,
  getDeployment,
  getDeploymentLogs,
  getDeploymentPricing,
  getDeploymentPublication,
  listDeployments,
  publishDeployment,
  reconcileDeploymentPublication,
  startDeployment,
  stopDeployment,
  updateDeployment,
  updateDeploymentPricing,
} from '../api'

const apiMocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

describe('deployment domain API scope', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    for (const request of Object.values(apiMocks)) {
      request.mockResolvedValue({ data: { success: true } })
    }
  })

  test('sends the selected domain when listing and creating deployments', async () => {
    await listDeployments({ domain: 'production', p: 2, page_size: 20 })
    expect(apiMocks.get).toHaveBeenCalledWith('/api/deployments/', {
      params: { domain: 'production', p: 2, page_size: 20 },
    })

    const deployment = {
      domain: 'staging' as const,
      name: 'staging-model',
      id: 'staging-model',
      code: 'test/model',
      image: 'vllm',
      resourceDefinition: {
        cpu: '4',
        memory: '16Gi',
        gpu: 1,
        gpuResourceKey: 'nvidia.com/gpu',
        gpuNodeLabelKey: '',
      },
      modelCacheSize: '80Gi',
      param: '',
      codes: [{ id: '', branch: 'main', path: '', token: '' }],
    }
    await createDeployment(deployment)
    expect(apiMocks.post).toHaveBeenCalledWith('/api/deployments/', deployment)
  })

  test('scopes detail, lifecycle, logs, and updates by domain', async () => {
    await getDeployment('staging', 'shared-id')
    await updateDeployment('staging', 'shared-id', {
      name: 'updated',
      image: 'vllm',
      param: '',
      modelCacheSize: '80Gi',
      resourceDefinition: {
        cpu: '4',
        memory: '16Gi',
        gpu: 1,
        gpuResourceKey: 'nvidia.com/gpu',
        gpuNodeLabelKey: '',
      },
    })
    await startDeployment('staging', 'shared-id')
    await stopDeployment('staging', 'shared-id')
    await getDeploymentLogs('staging', 'shared-id', { page: 3, size: 50 })
    await deleteDeployment('staging', 'shared-id')

    expect(apiMocks.get).toHaveBeenNthCalledWith(
      1,
      '/api/deployments/shared-id',
      {
        params: { domain: 'staging' },
      }
    )
    expect(apiMocks.put).toHaveBeenCalledWith(
      '/api/deployments/shared-id',
      expect.any(Object),
      { params: { domain: 'staging' } }
    )
    expect(apiMocks.post).toHaveBeenNthCalledWith(
      1,
      '/api/deployments/shared-id/start',
      undefined,
      { params: { domain: 'staging' } }
    )
    expect(apiMocks.post).toHaveBeenNthCalledWith(
      2,
      '/api/deployments/shared-id/stop',
      undefined,
      { params: { domain: 'staging' } }
    )
    expect(apiMocks.get).toHaveBeenNthCalledWith(
      2,
      '/api/deployments/shared-id/logs',
      { params: { page: 3, size: 50, domain: 'staging' } }
    )
    expect(apiMocks.delete).toHaveBeenCalledWith('/api/deployments/shared-id', {
      params: { domain: 'staging' },
    })
  })

  test('scopes publication, pricing, and reconciliation by domain', async () => {
    await getDeploymentPublication('production', 42)
    await publishDeployment('production', 42, {
      access_group: 'default',
      idempotency_key: 'publish-42',
    })
    await reconcileDeploymentPublication('production', 42)
    await getDeploymentPricing('production', 42)
    await updateDeploymentPricing('production', 42, {
      mode: 'per_token',
      input_price: 1,
      output_price: 2,
    })

    expect(apiMocks.get).toHaveBeenNthCalledWith(
      1,
      '/api/deployments/42/publication',
      { params: { domain: 'production' } }
    )
    expect(apiMocks.post).toHaveBeenNthCalledWith(
      1,
      '/api/deployments/42/publication',
      {
        access_group: 'default',
        idempotency_key: 'publish-42',
      },
      { params: { domain: 'production' } }
    )
    expect(apiMocks.post).toHaveBeenNthCalledWith(
      2,
      '/api/deployments/42/publication/reconcile',
      undefined,
      { params: { domain: 'production' } }
    )
    expect(apiMocks.get).toHaveBeenNthCalledWith(
      2,
      '/api/deployments/42/pricing',
      { params: { domain: 'production' } }
    )
    expect(apiMocks.put).toHaveBeenCalledWith(
      '/api/deployments/42/pricing',
      { mode: 'per_token', input_price: 1, output_price: 2 },
      { params: { domain: 'production' } }
    )
  })
})
