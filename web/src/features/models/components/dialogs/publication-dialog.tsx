/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, RefreshCw, Unlink } from 'lucide-react'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  addDeploymentPublicationBindings,
  getDeploymentPublication,
  publishDeployment,
  reconcileDeploymentPublication,
  removeDeploymentPublicationBinding,
  unpublishDeployment,
  updateDeploymentPublicationUpstreamModel,
} from '../../api'
import { deploymentsQueryKeys } from '../../lib'
import type { DeploymentDomain, NewPublicationToken } from '../../types'
import { DeploymentPricingPanel } from './deployment-pricing-panel'

const newTokenDefaults: NewPublicationToken = {
  user_id: 0,
  name: '',
  expired_time: -1,
  remain_quota: 0,
  unlimited_quota: true,
  model_limits_enabled: true,
  allow_ips: '',
  cross_group_retry: false,
}

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
  const [tokenIDs, setTokenIDs] = useState('')
  const [upstreamModel, setUpstreamModel] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [pricingConfigured, setPricingConfigured] = useState(false)
  const [createNewToken, setCreateNewToken] = useState(false)
  const [newToken, setNewToken] =
    useState<NewPublicationToken>(newTokenDefaults)
  const queryKey = deploymentsQueryKeys.publication(domain, deploymentId ?? '')
  const { data, isLoading } = useQuery({
    queryKey,
    queryFn: () => getDeploymentPublication(domain, deploymentId ?? ''),
    enabled: open && Boolean(deploymentId),
  })
  const publication = data?.data
  const parsedTokenIDs = () =>
    tokenIDs
      .split(',')
      .map((value) => Number(value.trim()))
      .filter((value) => Number.isInteger(value) && value > 0)
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
    setTokenIDs('')
    await refresh()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Publication and API keys')}</DialogTitle>
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
            {publication.warning ? (
              <p className='rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-300'>
                {t(publication.warning)}
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
            <section className='space-y-3 rounded-lg border p-4'>
              <h3 className='font-medium'>{t('Bound API keys')}</h3>
              {publication.bindings.map((binding) => (
                <div
                  key={binding.token_id}
                  className='flex items-center justify-between gap-3 text-sm'
                >
                  <div>
                    <div>
                      {binding.token_name} · #{binding.token_id}
                    </div>
                    <div className='text-muted-foreground font-mono'>
                      {binding.token_key}
                    </div>
                  </div>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={submitting}
                    onClick={() =>
                      void run(() =>
                        removeDeploymentPublicationBinding(
                          domain,
                          deploymentId ?? '',
                          binding.token_id
                        )
                      )
                    }
                  >
                    <Unlink className='size-4' />
                    {t('Unbind')}
                  </Button>
                </div>
              ))}
              <div className='flex gap-2'>
                <Input
                  value={tokenIDs}
                  onChange={(event) => setTokenIDs(event.target.value)}
                  placeholder={t('API key IDs, comma separated')}
                />
                <Button
                  disabled={submitting || parsedTokenIDs().length === 0}
                  onClick={() =>
                    void run(() =>
                      addDeploymentPublicationBindings(
                        domain,
                        deploymentId ?? '',
                        parsedTokenIDs()
                      )
                    )
                  }
                >
                  {t('Bind')}
                </Button>
              </div>
            </section>
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
                'This deployment is not published. Configure pricing first, then bind at least one API key.'
              )}
            </p>
            {!pricingConfigured ? (
              <p className='rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-300'>
                {t('Save model pricing before publishing.')}
              </p>
            ) : null}
            <div className='grid gap-4 md:grid-cols-2'>
              <div>
                <Label className='mb-2'>{t('Fixed access group')}</Label>
                <Input
                  value={group}
                  onChange={(event) => setGroup(event.target.value)}
                />
              </div>
              <div>
                <Label className='mb-2'>{t('API key IDs')}</Label>
                <Input
                  value={tokenIDs}
                  onChange={(event) => setTokenIDs(event.target.value)}
                  placeholder='1, 2'
                />
              </div>
            </div>
            <label className='flex items-center justify-between gap-3 rounded-lg border p-3 text-sm'>
              <span>{t('Create and bind a new API key')}</span>
              <Switch
                checked={createNewToken}
                onCheckedChange={setCreateNewToken}
              />
            </label>
            {createNewToken ? (
              <div className='grid gap-4 rounded-lg border p-4 md:grid-cols-2'>
                <PublicationTokenField label={t('Owner user ID')}>
                  <Input
                    type='number'
                    min={1}
                    value={newToken.user_id || ''}
                    onChange={(event) =>
                      setNewToken((current) => ({
                        ...current,
                        user_id: Number(event.target.value),
                      }))
                    }
                  />
                </PublicationTokenField>
                <PublicationTokenField label={t('API key name')}>
                  <Input
                    value={newToken.name}
                    onChange={(event) =>
                      setNewToken((current) => ({
                        ...current,
                        name: event.target.value,
                      }))
                    }
                  />
                </PublicationTokenField>
                <PublicationTokenField label={t('Expiration timestamp')}>
                  <Input
                    type='number'
                    value={newToken.expired_time}
                    onChange={(event) =>
                      setNewToken((current) => ({
                        ...current,
                        expired_time: Number(event.target.value),
                      }))
                    }
                  />
                </PublicationTokenField>
                {!newToken.unlimited_quota ? (
                  <PublicationTokenField label={t('Quota')}>
                    <Input
                      type='number'
                      min={0}
                      value={newToken.remain_quota}
                      onChange={(event) =>
                        setNewToken((current) => ({
                          ...current,
                          remain_quota: Number(event.target.value),
                        }))
                      }
                    />
                  </PublicationTokenField>
                ) : (
                  <div />
                )}
                <PublicationTokenToggle
                  label={t('Unlimited Quota')}
                  checked={newToken.unlimited_quota}
                  onCheckedChange={(checked) =>
                    setNewToken((current) => ({
                      ...current,
                      unlimited_quota: checked,
                    }))
                  }
                />
                <PublicationTokenToggle
                  label={t('Restrict key to bound models')}
                  checked={newToken.model_limits_enabled}
                  onCheckedChange={(checked) =>
                    setNewToken((current) => ({
                      ...current,
                      model_limits_enabled: checked,
                    }))
                  }
                />
                <PublicationTokenToggle
                  label={t('Cross-group retry')}
                  checked={newToken.cross_group_retry}
                  onCheckedChange={(checked) =>
                    setNewToken((current) => ({
                      ...current,
                      cross_group_retry: checked,
                    }))
                  }
                />
                <PublicationTokenField
                  label={t('IP Whitelist (supports CIDR)')}
                  className='md:col-span-2'
                >
                  <Textarea
                    rows={3}
                    value={newToken.allow_ips}
                    onChange={(event) =>
                      setNewToken((current) => ({
                        ...current,
                        allow_ips: event.target.value,
                      }))
                    }
                  />
                </PublicationTokenField>
              </div>
            ) : null}
            <div className='flex justify-end'>
              <Button
                disabled={
                  submitting ||
                  !pricingConfigured ||
                  !group.trim() ||
                  (parsedTokenIDs().length === 0 && !createNewToken) ||
                  (createNewToken &&
                    (newToken.user_id <= 0 || !newToken.name.trim()))
                }
                onClick={() =>
                  void run(() =>
                    publishDeployment(domain, deploymentId ?? '', {
                      access_group: group.trim(),
                      token_ids: parsedTokenIDs(),
                      idempotency_key: crypto.randomUUID(),
                      new_token: createNewToken
                        ? { ...newToken, name: newToken.name.trim() }
                        : undefined,
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

function PublicationTokenField({
  label,
  className,
  children,
}: {
  label: string
  className?: string
  children: React.ReactNode
}) {
  return (
    <div className={className}>
      <Label className='mb-2'>{label}</Label>
      {children}
    </div>
  )
}

function PublicationTokenToggle({
  label,
  checked,
  onCheckedChange,
}: {
  label: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <label className='flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm'>
      <span>{label}</span>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </label>
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
