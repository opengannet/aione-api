/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
/* oxlint-disable eslint/no-nested-ternary */
import { useQuery } from '@tanstack/react-query'
import { ExternalLink, Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import { getDeployment } from '../../api'
import { deploymentsQueryKeys } from '../../lib'
import type { DeploymentDomain } from '../../types'
import { deploymentStatusName } from '../deployments-columns'

export function ViewDetailsDialog({
  open,
  onOpenChange,
  deploymentId,
  domain,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  deploymentId: string | number | null
  domain: DeploymentDomain
}) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: deploymentsQueryKeys.detail(domain, deploymentId ?? ''),
    queryFn: () => {
      if (deploymentId === null) throw new Error('deployment ID is required')
      return getDeployment(domain, deploymentId)
    },
    enabled: open && deploymentId !== null,
  })
  const detail = data?.data
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Deployment details')}</DialogTitle>
          <DialogDescription>{deploymentId}</DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <Loader2 className='mx-auto my-12 size-8 animate-spin' />
        ) : !detail ? (
          <p className='text-destructive py-8 text-center'>
            {data?.message || t('Failed to fetch deployment details')}
          </p>
        ) : (
          <div className='space-y-5'>
            <Section
              title={t('Status')}
              rows={[
                [t('Status'), t(deploymentStatusName(detail.deploymentStatus))],
                [t('Substate'), String(detail.substate)],
                [t('Replicas'), String(detail.currentReplicas)],
                [t('Message'), detail.message || '-'],
              ]}
            />
            <Section
              title={t('Endpoint')}
              rows={[[t('URL'), detail.url || '-']]}
            >
              {detail.url ? (
                <a
                  className='text-primary inline-flex items-center gap-1 text-sm'
                  href={`${detail.url.replace(/\/$/, '')}/v1/models`}
                  target='_blank'
                  rel='noreferrer'
                >
                  {t('Open model validation endpoint')}
                  <ExternalLink className='size-4' />
                </a>
              ) : null}
            </Section>
            {detail.publication ? (
              <Section
                title={t('Publication')}
                rows={[
                  [t('Publication status'), detail.publication.phase],
                  [t('Reason'), detail.publication.reason_code || '-'],
                  [t('Fixed access group'), detail.publication.access_group],
                  [
                    t('Gateway channel ID'),
                    String(detail.publication.channel_id),
                  ],
                  [
                    t('Upstream model'),
                    detail.publication.upstream_model || '-',
                  ],
                  [
                    t('Bound API keys'),
                    detail.publication.bindings
                      .map((binding) => binding.token_name)
                      .join(', ') || '-',
                  ],
                  [
                    t('Pricing'),
                    detail.publication.pricing_configured
                      ? t('Configured')
                      : t('Missing'),
                  ],
                ]}
              />
            ) : null}
            <Section
              title={t('Configuration')}
              rows={[
                [t('Name'), detail.config.name],
                [t('Model code'), detail.config.code],
                [t('Container image'), detail.config.image],
                [t('Startup arguments'), detail.config.param || '-'],
              ]}
            />
            <Section
              title={t('Resources')}
              rows={[
                [t('CPU'), detail.config.resourceDefinition.cpu],
                [t('Memory'), detail.config.resourceDefinition.memory],
                [t('GPU'), String(detail.config.resourceDefinition.gpu)],
                [
                  t('GPU resource key'),
                  detail.config.resourceDefinition.gpuResourceKey || '-',
                ],
                [
                  t('GPU node label key'),
                  detail.config.resourceDefinition.gpuNodeLabelKey || '-',
                ],
              ]}
            />
            <Section
              title={t('Model cache')}
              rows={
                detail.config.modelCachePvc
                  ? [
                      [t('PVC'), detail.config.modelCachePvc.name],
                      [
                        t('Storage class'),
                        detail.config.modelCachePvc.storageClassName,
                      ],
                      [
                        t('Requested size'),
                        detail.config.modelCachePvc.requestedSize,
                      ],
                      [t('Capacity'), detail.config.modelCachePvc.capacity],
                      [
                        t('Expandable'),
                        detail.config.modelCachePvc.expandable
                          ? t('Yes')
                          : t('No'),
                      ],
                    ]
                  : [[t('PVC'), '-']]
              }
            />
            <Section
              title={t('Code source')}
              rows={detail.config.codes.flatMap((source) => [
                [t('Source address'), source.id],
                [
                  t('Branch / path'),
                  `${source.branch || '-'} / ${source.path || '-'}`,
                ],
                [
                  t('Access token'),
                  source.tokenConfigured
                    ? t('Configured (hidden)')
                    : t('Not configured'),
                ],
              ])}
            />
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function Section({
  title,
  rows,
  children,
}: {
  title: string
  rows: string[][]
  children?: ReactNode
}) {
  return (
    <section className='rounded-lg border p-4'>
      <h3 className='mb-3 font-medium'>{title}</h3>
      <dl className='grid gap-2 text-sm md:grid-cols-2'>
        {rows.map(([label, value]) => (
          <div key={`${label}-${value}`} className='min-w-0'>
            <dt className='text-muted-foreground'>{label}</dt>
            <dd className='font-mono break-words'>{value}</dd>
          </div>
        ))}
      </dl>
      {children ? <div className='mt-3'>{children}</div> : null}
    </section>
  )
}
