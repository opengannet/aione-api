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
import * as z from 'zod'

import type { DeploymentPricing, UpdateDeploymentPricing } from '../types'

export const simplePricingModes = ['free', 'per_token', 'per_request'] as const

export function createDeploymentPricingSchema(t: (key: string) => string) {
  return z
    .object({
      mode: z.enum(simplePricingModes),
      inputPrice: z.string(),
      outputPrice: z.string(),
      requestPrice: z.string(),
    })
    .superRefine((values, context) => {
      if (values.mode === 'per_token') {
        const inputPrice = Number(values.inputPrice)
        if (
          values.inputPrice.trim() === '' ||
          !Number.isFinite(inputPrice) ||
          inputPrice <= 0
        ) {
          context.addIssue({
            code: 'custom',
            path: ['inputPrice'],
            message: t('Input price must be greater than zero.'),
          })
        }
        const outputPrice = Number(values.outputPrice)
        if (
          values.outputPrice.trim() === '' ||
          !Number.isFinite(outputPrice) ||
          outputPrice < 0
        ) {
          context.addIssue({
            code: 'custom',
            path: ['outputPrice'],
            message: t('Output price must not be negative.'),
          })
        }
      }
      if (values.mode === 'per_request') {
        const requestPrice = Number(values.requestPrice)
        if (
          values.requestPrice.trim() === '' ||
          !Number.isFinite(requestPrice) ||
          requestPrice < 0
        ) {
          context.addIssue({
            code: 'custom',
            path: ['requestPrice'],
            message: t('Request price must not be negative.'),
          })
        }
      }
    })
}

export type DeploymentPricingFormValues = z.infer<
  ReturnType<typeof createDeploymentPricingSchema>
>

export function deploymentPricingFormValues(
  pricing?: DeploymentPricing
): DeploymentPricingFormValues {
  const mode = simplePricingModes.includes(
    pricing?.mode as (typeof simplePricingModes)[number]
  )
    ? (pricing?.mode as (typeof simplePricingModes)[number])
    : 'per_token'
  return {
    mode,
    inputPrice: pricing?.input_price?.toString() ?? '',
    outputPrice: pricing?.output_price?.toString() ?? '',
    requestPrice: pricing?.request_price?.toString() ?? '',
  }
}

export function deploymentPricingPayload(
  values: DeploymentPricingFormValues
): UpdateDeploymentPricing {
  if (values.mode === 'free') return { mode: 'free' }
  if (values.mode === 'per_request') {
    return { mode: 'per_request', request_price: Number(values.requestPrice) }
  }
  return {
    mode: 'per_token',
    input_price: Number(values.inputPrice),
    output_price: Number(values.outputPrice),
  }
}
