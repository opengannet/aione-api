/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { getDeployment, updateDeployment } from '../../api'
import { deploymentsQueryKeys } from '../../lib'
import type { DeploymentFormData } from '../../types'

type Editable = Pick<
  DeploymentFormData,
  'name' | 'image' | 'param' | 'modelCacheSize' | 'resourceDefinition'
>

export function UpdateConfigDialog({
  open,
  onOpenChange,
  deploymentId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  deploymentId: string | number | null
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [values, setValues] = useState<Editable>()
  const [saving, setSaving] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: deploymentsQueryKeys.detail(deploymentId ?? ''),
    queryFn: () => {
      if (deploymentId === null) throw new Error('deployment ID is required')
      return getDeployment(deploymentId)
    },
    enabled: open && deploymentId !== null,
  })
  const detail = data?.data
  useEffect(() => {
    if (!detail) return
    setValues({
      name: detail.config.name,
      image: detail.config.image,
      param: detail.config.param,
      modelCacheSize: detail.config.modelCachePvc?.requestedSize || '80Gi',
      resourceDefinition: detail.config.resourceDefinition,
    })
  }, [detail])
  const set = <K extends keyof Editable>(key: K, value: Editable[K]) =>
    setValues((current) => (current ? { ...current, [key]: value } : current))
  const setResource = <
    K extends keyof DeploymentFormData['resourceDefinition'],
  >(
    key: K,
    value: DeploymentFormData['resourceDefinition'][K]
  ) =>
    setValues((current) =>
      current
        ? {
            ...current,
            resourceDefinition: { ...current.resourceDefinition, [key]: value },
          }
        : current
    )
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!deploymentId || !values) return
    setSaving(true)
    const response = await updateDeployment(deploymentId, values)
    setSaving(false)
    if (!response.success) {
      toast.error(response.message || t('Update failed'))
      return
    }
    toast.success(t('Deployment updated and redeployment started'))
    onOpenChange(false)
    void queryClient.invalidateQueries({ queryKey: deploymentsQueryKeys.all })
  }
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Edit deployment')}</DialogTitle>
          <DialogDescription>
            {t('Saving runtime settings triggers a Flyte2 redeployment.')}
          </DialogDescription>
        </DialogHeader>
        {isLoading || !values || !detail ? (
          <Loader2 className='mx-auto my-12 size-8 animate-spin' />
        ) : (
          <form
            id='flyte2-edit-form'
            className='grid gap-4 md:grid-cols-2'
            onSubmit={submit}
          >
            <Field label={t('Application ID')}>
              <Input value={detail.id} disabled />
            </Field>
            <Field label={t('Model code')}>
              <Input value={detail.config.code} disabled />
            </Field>
            <Field label={t('Name')}>
              <Input
                value={values.name}
                onChange={(e) => set('name', e.target.value)}
              />
            </Field>
            <Field label={t('Image')}>
              <Input
                value={values.image}
                onChange={(e) => set('image', e.target.value)}
              />
            </Field>
            <Field label={t('CPU')}>
              <Input
                value={values.resourceDefinition.cpu}
                onChange={(e) => setResource('cpu', e.target.value)}
              />
            </Field>
            <Field label={t('Memory')}>
              <Input
                value={values.resourceDefinition.memory}
                onChange={(e) => setResource('memory', e.target.value)}
              />
            </Field>
            <Field label={t('GPU')}>
              <Input
                type='number'
                min={0}
                value={values.resourceDefinition.gpu}
                onChange={(e) => setResource('gpu', Number(e.target.value))}
              />
            </Field>
            <Field label={t('Model cache size')}>
              <Input
                value={values.modelCacheSize}
                onChange={(e) => set('modelCacheSize', e.target.value)}
              />
            </Field>
            <Field label={t('GPU resource key')}>
              <Input
                value={values.resourceDefinition.gpuResourceKey}
                onChange={(e) => setResource('gpuResourceKey', e.target.value)}
              />
            </Field>
            <Field label={t('GPU node label key')}>
              <Input
                value={values.resourceDefinition.gpuNodeLabelKey}
                onChange={(e) => setResource('gpuNodeLabelKey', e.target.value)}
              />
            </Field>
            <Field label={t('Startup arguments')} className='md:col-span-2'>
              <Textarea
                rows={5}
                value={values.param}
                onChange={(e) => set('param', e.target.value)}
              />
            </Field>
            <div className='space-y-3 rounded-lg border p-4 md:col-span-2'>
              <h3 className='font-medium'>{t('Code source (read-only)')}</h3>
              {detail.config.codes.length ? (
                detail.config.codes.map((source) => (
                  <div
                    key={`${source.id}-${source.path}`}
                    className='grid gap-3 md:grid-cols-3'
                  >
                    <Input value={source.id} disabled />
                    <Input value={source.branch || ''} disabled />
                    <Input
                      value={`${source.path || ''}${source.tokenConfigured ? ` · ${t('Token configured')}` : ''}`}
                      disabled
                    />
                  </div>
                ))
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {t('No code source configured')}
                </p>
              )}
            </div>
          </form>
        )}
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='flyte2-edit-form'
            disabled={!values || saving}
          >
            {saving ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Field({
  label,
  className,
  children,
}: {
  label: string
  className?: string
  children: ReactNode
}) {
  return (
    <div className={className}>
      <Label className='mb-2'>{label}</Label>
      {children}
    </div>
  )
}
