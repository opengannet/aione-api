/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import {
  getDeploymentPublication,
  publishDeployment,
  reconcileDeploymentPublication,
  unpublishDeployment,
  updateDeploymentPublicationUpstreamModel,
} from '../../api'
import { deploymentsQueryKeys } from '../../lib'
import type { DeploymentDomain } from '../../types'
import { DeploymentPricingPanel } from './deployment-pricing-panel'

export function PublicationDialog({
  open,
  onOpenChange,
  deploymentId,
  domain,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  deploymentId: string | null
  domain: DeploymentDomain
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [group, setGroup] = useState('aione')
  const [upstreamModel, setUpstreamModel] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [pricingConfigured, setPricingConfigured] = useState(false)
  const queryKey = deploymentsQueryKeys.publication(domain, deploymentId ?? '')
  const { data, isLoading } = useQuery({
    queryKey,
    queryFn: () => getDeploymentPublication(domain, deploymentId ?? ''),
    enabled: open && Boolean(deploymentId),
  })
  const publication = data?.data
  useEffect(() => {
    if (open) setPricingConfigured(false)
  }, [deploymentId, open])

  const refresh = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey }),
      queryClient.invalidateQueries({ queryKey: deploymentsQueryKeys.lists() }),
    ])
  }, [queryClient, queryKey])
  const run = async (
    action: () => Promise<{ success: boolean; message?: string }>
  ) => {
    setSubmitting(true)
    const response = await action()
    setSubmitting(false)
    if (!response.success) {
      toast.error(response.message || t('Operation failed'))
      return
    }
    toast.success(t('Operation successful'))
    await refresh()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Publication')}</DialogTitle>
          <DialogDescription>{deploymentId}</DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <Loader2 className='mx-auto my-10 size-8 animate-spin' />
        ) : null}
        {!isLoading && deploymentId ? (
          <DeploymentPricingPanel
            deploymentId={deploymentId}
            domain={domain}
            onStatusChange={setPricingConfigured}
            onSaved={refresh}
          />
        ) : null}
        {!isLoading && publication ? (
          <div className='space-y-5'>
            <div className='grid gap-3 rounded-lg border p-4 text-sm md:grid-cols-2'>
              <Value
                label={t('Publication status')}
                value={publication.phase}
              />
              <Value
                label={t('Reason')}
                value={publication.reason_code || '-'}
              />
              <Value
                label={t('Fixed access group')}
                value={publication.access_group}
              />
              <Value
                label={t('Gateway channel ID')}
                value={String(publication.channel_id)}
              />
              <Value
                label={t('Endpoint')}
                value={publication.endpoint || '-'}
              />
              <Value
                label={t('Upstream model')}
                value={publication.upstream_model || '-'}
              />
            </div>
            {publication.last_error ? (
              <p className='text-destructive rounded-lg border p-3 text-sm'>
                {publication.last_error}
              </p>
            ) : null}
            {publication.reason_code === 'upstream_model_required' ? (
              <div className='grid gap-2'>
                <Label>{t('Upstream model')}</Label>
                <div className='flex gap-2'>
                  <Input
                    value={upstreamModel}
                    onChange={(event) => setUpstreamModel(event.target.value)}
                  />
                  <Button
                    disabled={submitting || !upstreamModel.trim()}
                    onClick={() =>
                      void run(() =>
                        updateDeploymentPublicationUpstreamModel(
                          domain,
                          deploymentId ?? '',
                          upstreamModel.trim()
                        )
                      )
                    }
                  >
                    {t('Save')}
                  </Button>
                </div>
              </div>
            ) : null}
            <div className='flex flex-wrap justify-end gap-2'>
              <Button
                variant='outline'
                disabled={submitting}
                onClick={() =>
                  void run(() =>
                    reconcileDeploymentPublication(domain, deploymentId ?? '')
                  )
                }
              >
                <RefreshCw className='size-4' />
                {t('Reconcile')}
              </Button>
              <Button
                variant='destructive'
                disabled={submitting}
                onClick={() =>
                  void run(() =>
                    unpublishDeployment(domain, deploymentId ?? '')
                  )
                }
              >
                {t('Unpublish')}
              </Button>
            </div>
          </div>
        ) : null}
        {!isLoading && !publication ? (
          <div className='space-y-4'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'This deployment is not published. Configure pricing before publishing.'
              )}
            </p>
            {!pricingConfigured ? (
              <p className='rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-300'>
                {t('Save model pricing before publishing.')}
              </p>
            ) : null}
            <div>
              <Label className='mb-2'>{t('Fixed access group')}</Label>
              <Input
                value={group}
                onChange={(event) => setGroup(event.target.value)}
              />
            </div>
            <div className='flex justify-end'>
              <Button
                disabled={submitting || !pricingConfigured || !group.trim()}
                onClick={() =>
                  void run(() =>
                    publishDeployment(domain, deploymentId ?? '', {
                      access_group: group.trim(),
                      idempotency_key: crypto.randomUUID(),
                    })
                  )
                }
              >
                {t('Publish')}
              </Button>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function Value({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground'>{label}</div>
      <div className='font-mono break-words'>{value}</div>
    </div>
  )
}
