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
import { z } from 'zod'

// ============================================================================
// Model Types
// ============================================================================

/**
 * Bound channel information
 */
export interface BoundChannel {
  name: string
  type: number
}

/**
 * Model entity from API
 */
export interface Model {
  id: number
  model_name: string
  description?: string
  icon?: string
  tags?: string
  vendor_id?: number
  endpoints?: string
  status: number
  sync_official: number
  created_time: number
  updated_time: number
  name_rule: number
  // Runtime fields
  bound_channels?: BoundChannel[]
  enable_groups?: string[]
  quota_types?: number[]
  matched_models?: string[]
  matched_count?: number
}

/**
 * Vendor entity from API
 */
export interface Vendor {
  id: number
  name: string
  description?: string
  icon?: string
  status: number
  created_time: number
  updated_time: number
}

/**
 * Prefill group entity
 */
export interface PrefillGroup {
  id: number
  name: string
  type: 'model' | 'tag' | 'endpoint'
  items: string | string[]
  description?: string
}

// ============================================================================
// API Request/Response Types
// ============================================================================

/**
 * Get models list parameters
 */
export interface GetModelsParams {
  p?: number
  page_size?: number
  vendor?: string // vendor ID to filter by
  status?: string // filter by status
  sync_official?: string // filter by sync_official status
}

/**
 * Search models parameters
 */
export interface SearchModelsParams {
  keyword?: string
  vendor?: string // vendor ID to filter by
  status?: string // filter by status
  sync_official?: string // filter by sync_official status
  p?: number
  page_size?: number
}

/**
 * Get models response
 */
export interface GetModelsResponse {
  success: boolean
  message?: string
  data?: {
    items: Model[]
    total: number
    page: number
    page_size: number
    vendor_counts?: Record<string, number>
  }
}

/**
 * Get model detail response
 */
export interface GetModelResponse {
  success: boolean
  message?: string
  data?: Model
}

/**
 * Get vendors response
 */
export interface GetVendorsResponse {
  success: boolean
  message?: string
  data?: {
    items: Vendor[]
    total: number
    page: number
    page_size: number
  }
}

/**
 * Get vendor response
 */
export interface GetVendorResponse {
  success: boolean
  message?: string
  data?: Vendor
}

/**
 * Sync diff data
 */
export interface SyncDiffData {
  missing?: Array<{
    model_name: string
    vendor?: string
    [key: string]: unknown
  }>
  conflicts?: Array<{
    model_name: string
    local?: Partial<Model>
    upstream?: Partial<Model>
    fields?: Array<{
      field: string
      local?: unknown
      upstream?: unknown
    }>
    [key: string]: unknown
  }>
}

export interface SyncOverwritePayload {
  model_name: string
  fields: string[]
}

/**
 * Sync upstream response
 */
export interface SyncUpstreamResponse {
  success: boolean
  message?: string
  data?: {
    created_models?: number
    updated_models?: number
    created_vendors?: number
    skipped_models?: string[]
  }
}

/**
 * Preview upstream diff response
 */
export interface PreviewUpstreamDiffResponse {
  success: boolean
  message?: string
  data?: SyncDiffData
}

/**
 * Missing models response
 */
export interface MissingModelsResponse {
  success: boolean
  message?: string
  data?: string[]
}

/**
 * Prefill groups response
 */
export interface PrefillGroupsResponse {
  success: boolean
  message?: string
  data?: PrefillGroup[]
}

// ============================================================================
// Form Data Types
// ============================================================================

/**
 * Model form schema
 */
export const modelFormSchema = z.object({
  id: z.number().optional(),
  model_name: z.string().min(1, 'Model name is required'),
  description: z.string().default(''),
  icon: z.string().default(''),
  tags: z.array(z.string()).default([]),
  vendor_id: z.number().optional(),
  endpoints: z.string().default(''),
  name_rule: z.number().min(0).max(3).default(0),
  status: z.boolean().default(true),
  sync_official: z.boolean().default(true),
})

export type ModelFormValues = z.infer<typeof modelFormSchema>

/**
 * Vendor form schema
 */
export const vendorFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Vendor name is required'),
  description: z.string().default(''),
  icon: z.string().default(''),
  status: z.number().default(1),
})

export type VendorFormValues = z.infer<typeof vendorFormSchema>

/**
 * Prefill group form schema
 */
export const prefillGroupFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Group name is required'),
  description: z.string().optional(),
  type: z.enum(['model', 'tag', 'endpoint']),
  items: z.union([z.string(), z.array(z.string())]),
})

export type PrefillGroupFormValues = z.infer<typeof prefillGroupFormSchema>

// ============================================================================
// Utility Types
// ============================================================================

/**
 * Name rule type
 */
export type NameRule = 0 | 1 | 2 | 3 // exact, prefix, contains, suffix

/**
 * Model status type
 */
export type ModelStatus = 0 | 1 // disabled, enabled

/**
 * Quota type
 */
export type QuotaType = 0 | 1 // usage-based, per-call

/**
 * Sync locale
 */
export type SyncLocale = 'zh' | 'en' | 'ja'

/**
 * Sync upstream source
 */
export type SyncSource = 'official' | 'config'

// ============================================================================
// Model Deployments Types
// ============================================================================

/**
 * Model tab type
 */
export type ModelTabCategory = 'metadata' | 'deployments'

/**
 * Deployment entity from API
 */
export interface Deployment {
  id: string
  org: string
  project: string
  domain: string
  name: string
  code: string
  type: 'VLLM'
  image: string
  deploymentStatus: number
  substate: number
  message: string
  currentReplicas: number
  url: string
  createdAt: string
  updatedAt: string
  publication?: FlytePublication
}

export interface FlytePublicationBinding {
  token_id: number
  token_name: string
  token_key: string
  managed_permission_added: boolean
  restricted: boolean
}

export interface FlytePublication {
  id: number
  deployment_id: string
  model_code: string
  endpoint: string
  upstream_model: string
  phase: 'pending' | 'published' | 'drifted' | 'cleanup_pending'
  reason_code: string
  last_error: string
  access_group: string
  organization: string
  channel_id: number
  pricing_configured: boolean
  bindings: FlytePublicationBinding[]
  warning?: string
}

export interface NewPublicationToken {
  user_id: number
  name: string
  expired_time: number
  remain_quota: number
  unlimited_quota: boolean
  model_limits_enabled: boolean
  allow_ips: string
  cross_group_retry: boolean
}

export interface DeploymentResourceDefinition {
  cpu: string
  memory: string
  gpu: number
  gpuResourceKey?: string
  gpuNodeLabelKey?: string
}

export interface DeploymentCodeSource {
  id: string
  branch?: string
  path?: string
  token?: string
  tokenConfigured?: boolean
}

export interface DeploymentFormData {
  name: string
  id: string
  code: string
  image: string
  param: string
  modelCacheSize: string
  resourceDefinition: DeploymentResourceDefinition
  codes: DeploymentCodeSource[]
}

export interface DeploymentDetail extends Deployment {
  desiredState: number
  config: {
    name: string
    code: string
    image: string
    param: string
    codes: DeploymentCodeSource[]
    resourceDefinition: DeploymentResourceDefinition
    modelCachePvc?: {
      name: string
      storageClassName: string
      requestedSize: string
      capacity: string
      expandable: boolean
    }
  }
}

/**
 * Deployment settings response
 */
export interface DeploymentSettingsResponse {
  success: boolean
  message?: string
  data?: {
    provider: 'flyte2'
    enabled: boolean
    base_url: string
    project: string
    domain: string
    configured: boolean
    publication_enabled: boolean
  }
}

/**
 * List deployments response
 */
export interface ListDeploymentsResponse {
  success: boolean
  message?: string
  data?: {
    items?: Deployment[]
    total?: number
    page?: number
    pageSize?: number
  }
}

/**
 * Deployment logs response
 */
export interface DeploymentLogsResponse {
  success: boolean
  message?: string
  data?: {
    items?: Array<{
      timestamp: string
      message: string
    }>
    total?: number
    page?: number
    size?: number
  }
}
