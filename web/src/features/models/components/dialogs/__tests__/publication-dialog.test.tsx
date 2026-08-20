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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { PublicationDialog } from '../publication-dialog'

const apiMocks = vi.hoisted(() => ({
  getDeploymentPricing: vi.fn(),
  getDeploymentPublication: vi.fn(),
  publishDeployment: vi.fn(),
}))

vi.mock('../../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../api')>()),
  getDeploymentPricing: apiMocks.getDeploymentPricing,
  getDeploymentPublication: apiMocks.getDeploymentPublication,
  publishDeployment: apiMocks.publishDeployment,
}))

describe('PublicationDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getDeploymentPublication.mockResolvedValue({
      success: true,
      data: null,
    })
    apiMocks.getDeploymentPricing.mockResolvedValue({
      success: true,
      data: {
        deployment_id: 'deployment-1',
        model_code: 'model-1',
        configured: true,
        mode: 'free',
        advanced_only: false,
        advanced_page_url: '/pricing',
      },
    })
    apiMocks.publishDeployment.mockResolvedValue({ success: true })
  })

  test('publishes without showing or submitting API key controls', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <PublicationDialog
          open
          onOpenChange={() => undefined}
          deploymentId='deployment-1'
          domain='production'
        />
      </QueryClientProvider>
    )

    expect(await screen.findByRole('dialog')).toBeVisible()
    expect(screen.getByRole('heading', { name: 'Publication' })).toBeVisible()
    expect(screen.queryByText('API key IDs')).not.toBeInTheDocument()
    expect(screen.queryByText('Bound API keys')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Create and bind a new API key')
    ).not.toBeInTheDocument()

    const publishButton = await screen.findByRole('button', {
      name: 'Publish',
    })
    await waitFor(() => expect(publishButton).toBeEnabled())
    fireEvent.click(publishButton)

    await waitFor(() => expect(apiMocks.publishDeployment).toHaveBeenCalled())
    const payload = apiMocks.publishDeployment.mock.calls[0]?.[2]
    expect(payload).toEqual({
      access_group: 'aione',
      idempotency_key: expect.any(String),
    })
    expect(payload).not.toHaveProperty('token_ids')
    expect(payload).not.toHaveProperty('new_token')
  })
})
