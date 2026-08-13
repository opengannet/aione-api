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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { getDeploymentPricing, updateDeploymentPricing } from '../../api'
import { deploymentsQueryKeys } from '../../lib'
import {
  createDeploymentPricingSchema,
  deploymentPricingFormValues,
  deploymentPricingPayload,
  type DeploymentPricingFormValues,
} from '../../lib/deployment-pricing'

type DeploymentPricingPanelProps = {
  deploymentId: string
  onStatusChange: (configured: boolean) => void
  onSaved: () => Promise<void>
}

export function DeploymentPricingPanel(props: DeploymentPricingPanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const onStatusChange = props.onStatusChange
  const [editing, setEditing] = useState(false)
  const queryKey = deploymentsQueryKeys.pricing(props.deploymentId)
  const pricingQuery = useQuery({
    queryKey,
    queryFn: () => getDeploymentPricing(props.deploymentId),
  })
  const pricing = pricingQuery.data?.data
  const schema = useMemo(() => createDeploymentPricingSchema(t), [t])
  const form = useForm<DeploymentPricingFormValues>({
    resolver: zodResolver(schema),
    defaultValues: deploymentPricingFormValues(),
  })
  const mode = form.watch('mode')

  useEffect(() => {
    form.reset(deploymentPricingFormValues(pricing))
  }, [form, pricing])

  useEffect(() => {
    onStatusChange(pricing?.configured === true)
  }, [pricing?.configured, onStatusChange])

  const saveMutation = useMutation({
    mutationFn: (values: DeploymentPricingFormValues) =>
      updateDeploymentPricing(
        props.deploymentId,
        deploymentPricingPayload(values)
      ),
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save model pricing'))
        return
      }
      toast.success(t('Model pricing saved'))
      setEditing(false)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey }),
        props.onSaved(),
      ])
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save model pricing'))
    },
  })

  if (pricingQuery.isLoading) {
    return (
      <div className='flex justify-center rounded-lg border p-6'>
        <Loader2 className='size-5 animate-spin' />
      </div>
    )
  }

  if (!pricingQuery.data?.success || !pricing) {
    return (
      <div className='border-destructive/40 text-destructive rounded-lg border p-3 text-sm'>
        {pricingQuery.data?.message || t('Failed to load model pricing')}
      </div>
    )
  }

  return (
    <section className='space-y-4 rounded-lg border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <h3 className='font-medium'>{t('Model pricing')}</h3>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Model code')}:{' '}
            <span className='font-mono'>{pricing.model_code}</span>
          </p>
        </div>
        <div className='flex items-center gap-2'>
          <StatusBadge
            label={pricing.configured ? t('Configured') : t('Missing')}
            variant={pricing.configured ? 'success' : 'danger'}
            size='sm'
            copyable={false}
          />
          {!pricing.advanced_only ? (
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => setEditing((value) => !value)}
            >
              {editing ? t('Cancel') : t('Configure pricing')}
            </Button>
          ) : null}
        </div>
      </div>

      {pricing.advanced_only ? (
        <div className='bg-muted/40 space-y-3 rounded-md p-3 text-sm'>
          <p>
            {t(
              'This model uses advanced pricing. Edit it on the model pricing settings page.'
            )}
          </p>
          <Button
            type='button'
            size='sm'
            variant='outline'
            render={<a href={pricing.advanced_page_url} />}
          >
            {t('Open advanced pricing')}
          </Button>
        </div>
      ) : null}

      {editing && !pricing.advanced_only ? (
        <form
          className='space-y-4 border-t pt-4'
          onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
        >
          <div className='space-y-2'>
            <Label>{t('Pricing method')}</Label>
            <div className='flex flex-wrap gap-2'>
              {(
                [
                  ['free', t('Free')],
                  ['per_token', t('Per token')],
                  ['per_request', t('Per request')],
                ] as const
              ).map(([value, label]) => (
                <Button
                  key={value}
                  type='button'
                  size='sm'
                  variant={mode === value ? 'default' : 'outline'}
                  aria-pressed={mode === value}
                  onClick={() => form.setValue('mode', value)}
                >
                  {label}
                </Button>
              ))}
            </div>
          </div>

          {mode === 'per_token' ? (
            <div className='grid gap-4 md:grid-cols-2'>
              <PricingInput
                id='deployment-input-price'
                label={t('Input price (USD / 1M tokens)')}
                error={form.formState.errors.inputPrice?.message}
                inputProps={form.register('inputPrice')}
              />
              <PricingInput
                id='deployment-output-price'
                label={t('Output price (USD / 1M tokens)')}
                error={form.formState.errors.outputPrice?.message}
                inputProps={form.register('outputPrice')}
              />
            </div>
          ) : null}

          {mode === 'per_request' ? (
            <PricingInput
              id='deployment-request-price'
              label={t('Request price (USD / request)')}
              error={form.formState.errors.requestPrice?.message}
              inputProps={form.register('requestPrice')}
            />
          ) : null}

          {mode === 'free' ? (
            <p className='text-muted-foreground text-sm'>
              {t('Requests to this model will not consume quota.')}
            </p>
          ) : null}

          <div className='flex justify-end'>
            <Button type='submit' disabled={saveMutation.isPending}>
              {saveMutation.isPending ? (
                <Loader2 className='size-4 animate-spin' />
              ) : null}
              {t('Save pricing')}
            </Button>
          </div>
        </form>
      ) : null}
    </section>
  )
}

type PricingInputProps = {
  id: string
  label: string
  error?: string
  inputProps: React.ComponentProps<'input'>
}

function PricingInput(props: PricingInputProps) {
  return (
    <div className='space-y-2'>
      <Label htmlFor={props.id}>{props.label}</Label>
      <Input
        {...props.inputProps}
        id={props.id}
        type='number'
        min='0'
        step='any'
        aria-invalid={Boolean(props.error)}
      />
      {props.error ? (
        <p className='text-destructive text-sm'>{props.error}</p>
      ) : null}
    </div>
  )
}
