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
import axios from 'axios'

import { api } from '@/lib/api'

import type {
  GetModelsParams,
  GetModelsResponse,
  GetModelResponse,
  GetVendorsResponse,
  GetVendorResponse,
  Model,
  Vendor,
  SearchModelsParams,
  SyncUpstreamResponse,
  PreviewUpstreamDiffResponse,
  MissingModelsResponse,
  PrefillGroupsResponse,
  SyncLocale,
  SyncSource,
  SyncOverwritePayload,
  DeploymentSettingsResponse,
  ListDeploymentsResponse,
  DeploymentDetail,
  DeploymentFormData,
  DeploymentLogsResponse,
} from './types'

// ============================================================================
// Model CRUD Operations
// ============================================================================

/**
 * Get paginated list of models
 */
export async function getModels(
  params: GetModelsParams = {}
): Promise<GetModelsResponse> {
  const res = await api.get('/api/models/', { params })
  return res.data
}

/**
 * Search models with filters
 */
export async function searchModels(
  params: SearchModelsParams
): Promise<GetModelsResponse> {
  const res = await api.get('/api/models/search', { params })
  return res.data
}

/**
 * Get single model by ID
 */
export async function getModel(id: number): Promise<GetModelResponse> {
  const res = await api.get(`/api/models/${id}`)
  return res.data
}

/**
 * Create new model
 */
export async function createModel(
  data: Partial<Model>
): Promise<{ success: boolean; message?: string; data?: Model }> {
  const res = await api.post('/api/models/', data)
  return res.data
}

/**
 * Update existing model
 */
export async function updateModel(
  data: Partial<Model> & { id: number }
): Promise<{ success: boolean; message?: string; data?: Model }> {
  const res = await api.put('/api/models/', data)
  return res.data
}

/**
 * Update model status only
 */
export async function updateModelStatus(
  id: number,
  status: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/models/?status_only=true', { id, status })
  return res.data
}

/**
 * Delete model
 */
export async function deleteModel(
  id: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/models/${id}`)
  return res.data
}

// ============================================================================
// Vendor Management
// ============================================================================

/**
 * Get paginated list of vendors
 */
export async function getVendors(params?: {
  p?: number
  page_size?: number
}): Promise<GetVendorsResponse> {
  const res = await api.get('/api/vendors/', {
    params: params || { page_size: 1000 },
  })
  return res.data
}

/**
 * Search vendors
 */
export async function searchVendors(params: {
  keyword?: string
  p?: number
  page_size?: number
}): Promise<GetVendorsResponse> {
  const res = await api.get('/api/vendors/search', { params })
  return res.data
}

/**
 * Get single vendor by ID
 */
export async function getVendor(id: number): Promise<GetVendorResponse> {
  const res = await api.get(`/api/vendors/${id}`)
  return res.data
}

/**
 * Create new vendor
 */
export async function createVendor(
  data: Partial<Vendor>
): Promise<{ success: boolean; message?: string; data?: Vendor }> {
  const res = await api.post('/api/vendors/', data)
  return res.data
}

/**
 * Update existing vendor
 */
export async function updateVendor(
  data: Partial<Vendor> & { id: number }
): Promise<{ success: boolean; message?: string; data?: Vendor }> {
  const res = await api.put('/api/vendors/', data)
  return res.data
}

/**
 * Delete vendor
 */
export async function deleteVendor(
  id: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/vendors/${id}`)
  return res.data
}

// ============================================================================
// Sync Operations
// ============================================================================

/**
 * Sync upstream models (missing only or with overwrite)
 */
export async function syncUpstream(params?: {
  locale?: SyncLocale
  source?: SyncSource
  overwrite?: SyncOverwritePayload[]
}): Promise<SyncUpstreamResponse> {
  const res = await api.post('/api/models/sync_upstream', params)
  return res.data
}

/**
 * Preview upstream diff
 */
export async function previewUpstreamDiff(params?: {
  locale?: SyncLocale
  source?: SyncSource
}): Promise<PreviewUpstreamDiffResponse> {
  const searchParams = new URLSearchParams()
  if (params?.locale) {
    searchParams.set('locale', params.locale)
  }
  if (params?.source) {
    searchParams.set('source', params.source)
  }
  const queryString = searchParams.toString()
  const url = queryString
    ? `/api/models/sync_upstream/preview?${queryString}`
    : '/api/models/sync_upstream/preview'
  const res = await api.get(url)
  return res.data
}

/**
 * Apply upstream overwrite
 */
export async function applyUpstreamOverwrite(params: {
  overwrite: SyncOverwritePayload[]
  locale?: SyncLocale
  source?: SyncSource
}): Promise<SyncUpstreamResponse> {
  return syncUpstream(params)
}

// ============================================================================
// Utility Operations
// ============================================================================

/**
 * Get missing models (used but not configured)
 */
export async function getMissingModels(): Promise<MissingModelsResponse> {
  const res = await api.get('/api/models/missing')
  return res.data
}

/**
 * Get prefill groups
 */
export async function getPrefillGroups(
  type?: 'model' | 'tag' | 'endpoint'
): Promise<PrefillGroupsResponse> {
  const res = await api.get('/api/prefill_group', {
    params: type ? { type } : undefined,
  })
  return res.data
}

/**
 * Create prefill group
 */
export async function createPrefillGroup(data: {
  name: string
  type: 'model' | 'tag' | 'endpoint'
  items: string | string[]
  description?: string
}): Promise<{ success: boolean; message?: string }> {
  const res = await api.post('/api/prefill_group', data)
  return res.data
}

/**
 * Update prefill group
 */
export async function updatePrefillGroup(data: {
  id: number
  type?: 'model' | 'tag' | 'endpoint'
  name?: string
  items?: string | string[]
  description?: string
}): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/prefill_group', data)
  return res.data
}

/**
 * Delete prefill group
 */
export async function deletePrefillGroup(
  id: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/prefill_group/${id}`)
  return res.data
}

// Model deployment operations
export async function getDeploymentSettings(): Promise<DeploymentSettingsResponse> {
  const res = await api.get('/api/deployments/settings')
  return res.data
}

export async function updateDeploymentSettings(data: {
  enabled: boolean
  base_url: string
  project: string
  api_key?: string
  clear_api_key?: boolean
  publication_enabled?: boolean
}): Promise<DeploymentSettingsResponse> {
  const res = await api.put('/api/deployments/settings', data)
  return res.data
}

export async function testDeploymentConnection(
  data: {
    base_url?: string
    project?: string
    api_key?: string
  } = {}
): Promise<{
  success: boolean
  message?: string
  data?: {
    connected: boolean
    model_counts: Record<import('./types').DeploymentDomain, number>
    org: string
  }
}> {
  const config = { skipErrorHandler: true } as unknown as Parameters<
    typeof api.post
  >[2]
  const res = await api.post(
    '/api/deployments/settings/test-connection',
    data,
    config
  )
  return res.data
}

export async function listDeployments(params: {
  domain: import('./types').DeploymentDomain
  p?: number
  page_size?: number
  status?: string
  keyword?: string
}): Promise<ListDeploymentsResponse> {
  const res = await api.get('/api/deployments/', { params })
  return res.data
}

export async function getDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number
): Promise<{
  success: boolean
  message?: string
  data?: DeploymentDetail
}> {
  const res = await api.get(`/api/deployments/${id}`, { params: { domain } })
  return res.data
}

export async function deleteDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/deployments/${id}`, {
    params: { domain },
  })
  return res.data
}

export async function getDeploymentLogs(
  domain: import('./types').DeploymentDomain,
  deploymentId: string | number,
  params: { page?: number; size?: number }
): Promise<DeploymentLogsResponse> {
  const res = await api.get(`/api/deployments/${deploymentId}/logs`, {
    params: { ...params, domain },
  })
  return res.data
}

export async function createDeployment(data: DeploymentFormData): Promise<{
  success: boolean
  message?: string
  data?: DeploymentDetail
}> {
  const res = await api.post('/api/deployments/', data)
  return res.data
}

export async function updateDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number,
  data: Pick<
    DeploymentFormData,
    'name' | 'image' | 'param' | 'modelCacheSize' | 'resourceDefinition'
  >
): Promise<{
  success: boolean
  message?: string
  data?: DeploymentDetail
}> {
  const res = await api.put(`/api/deployments/${id}`, data, {
    params: { domain },
  })
  return res.data
}

export async function startDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number
): Promise<{
  success: boolean
  message?: string
}> {
  const res = await api.post(`/api/deployments/${id}/start`, undefined, {
    params: { domain },
  })
  return res.data
}

export async function stopDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number
): Promise<{
  success: boolean
  message?: string
}> {
  const res = await api.post(`/api/deployments/${id}/stop`, undefined, {
    params: { domain },
  })
  return res.data
}

export async function getDeploymentPublication(
  domain: import('./types').DeploymentDomain,
  id: string | number
) {
  const res = await api.get(`/api/deployments/${id}/publication`, {
    params: { domain },
  })
  return res.data as {
    success: boolean
    message?: string
    data?: import('./types').FlytePublication | null
  }
}

export async function getDeploymentPricing(
  domain: import('./types').DeploymentDomain,
  id: string | number
) {
  const res = await api.get(`/api/deployments/${id}/pricing`, {
    params: { domain },
  })
  return res.data as {
    success: boolean
    message?: string
    data?: import('./types').DeploymentPricing
  }
}

export async function updateDeploymentPricing(
  domain: import('./types').DeploymentDomain,
  id: string | number,
  data: import('./types').UpdateDeploymentPricing
) {
  try {
    const res = await api.put(`/api/deployments/${id}/pricing`, data, {
      params: { domain },
    })
    return res.data as {
      success: boolean
      message?: string
      data?: import('./types').DeploymentPricing
    }
  } catch (error: unknown) {
    if (axios.isAxiosError(error) && error.response?.data) {
      return error.response.data as { success: boolean; message?: string }
    }
    throw error
  }
}

export async function publishDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number,
  data: {
    access_group: string
    token_ids: number[]
    idempotency_key: string
    upstream_model?: string
    new_token?: import('./types').NewPublicationToken
  }
) {
  try {
    const res = await api.post(`/api/deployments/${id}/publication`, data, {
      params: { domain },
    })
    return res.data as {
      success: boolean
      message?: string
      data?: import('./types').FlytePublication & {
        created_token?: { id: number; key: string; name: string }
      }
    }
  } catch (error: unknown) {
    if (axios.isAxiosError(error) && error.response?.data) {
      return error.response.data as { success: boolean; message?: string }
    }
    throw error
  }
}

export async function unpublishDeployment(
  domain: import('./types').DeploymentDomain,
  id: string | number
) {
  const res = await api.delete(`/api/deployments/${id}/publication`, {
    params: { domain },
  })
  return res.data as { success: boolean; message?: string }
}

export async function addDeploymentPublicationBindings(
  domain: import('./types').DeploymentDomain,
  id: string | number,
  tokenIds: number[]
) {
  const res = await api.post(
    `/api/deployments/${id}/publication/bindings`,
    {
      token_ids: tokenIds,
      idempotency_key: crypto.randomUUID(),
    },
    { params: { domain } }
  )
  return res.data as { success: boolean; message?: string }
}

export async function removeDeploymentPublicationBinding(
  domain: import('./types').DeploymentDomain,
  id: string | number,
  tokenId: number
) {
  const res = await api.delete(
    `/api/deployments/${id}/publication/bindings/${tokenId}`,
    { params: { domain } }
  )
  return res.data as { success: boolean; message?: string }
}

export async function reconcileDeploymentPublication(
  domain: import('./types').DeploymentDomain,
  id: string | number
) {
  const res = await api.post(
    `/api/deployments/${id}/publication/reconcile`,
    undefined,
    { params: { domain } }
  )
  return res.data as { success: boolean; message?: string }
}

export async function updateDeploymentPublicationUpstreamModel(
  domain: import('./types').DeploymentDomain,
  id: string | number,
  upstreamModel: string
) {
  const res = await api.put(
    `/api/deployments/${id}/publication/upstream-model`,
    { upstream_model: upstreamModel },
    { params: { domain } }
  )
  return res.data as { success: boolean; message?: string }
}
