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
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'

import {
  createDeployment,
  getDeploymentSettings,
  publishDeployment,
} from '../../api'
import { deploymentsQueryKeys } from '../../lib'
import {
  DEPLOYMENT_DOMAINS,
  type DeploymentDomain,
  type DeploymentFormData,
} from '../../types'

const defaults: DeploymentFormData = {
  domain: 'development',
  name: '',
  id: '',
  code: '',
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
  codes: [{ id: '', branch: 'main', path: '', token: '' }],
}

export function CreateDeploymentDrawer({
  open,
  onOpenChange,
  currentDomain,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentDomain: DeploymentDomain
  onCreated: (domain: DeploymentDomain) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [values, setValues] = useState<DeploymentFormData>(defaults)
  const [submitting, setSubmitting] = useState(false)
  const [publishAfterCreate, setPublishAfterCreate] = useState(false)
  const [accessGroup, setAccessGroup] = useState('aione')
  const { data: settingsResponse } = useQuery({
    queryKey: [...deploymentsQueryKeys.all, 'settings'],
    queryFn: getDeploymentSettings,
    enabled: open,
    staleTime: 30_000,
  })
  const publicationEnabled =
    settingsResponse?.data?.publication_enabled === true
  const deploymentDomains =
    settingsResponse?.data?.domains ?? DEPLOYMENT_DOMAINS
  useEffect(() => {
    if (open) {
      setPublishAfterCreate(publicationEnabled)
      setValues((current) => ({ ...current, domain: currentDomain }))
    }
  }, [currentDomain, open, publicationEnabled])
  const resource = values.resourceDefinition
  const source = values.codes[0]
  const set = <K extends keyof DeploymentFormData>(
    key: K,
    value: DeploymentFormData[K]
  ) => setValues((current) => ({ ...current, [key]: value }))
  const setResource = <
    K extends keyof DeploymentFormData['resourceDefinition'],
  >(
    key: K,
    value: DeploymentFormData['resourceDefinition'][K]
  ) =>
    setValues((current) => ({
      ...current,
      resourceDefinition: { ...current.resourceDefinition, [key]: value },
    }))
  const setSource = (key: keyof typeof source, value: string) =>
    setValues((current) => ({
      ...current,
      codes: [{ ...current.codes[0], [key]: value }],
    }))

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!values.name.trim() || !values.id.trim() || !values.code.trim()) {
      toast.error(t('Name, application ID, and model code are required.'))
      return
    }
    if (!/^[1-9]\d*Gi$/.test(values.modelCacheSize)) {
      toast.error(t('Model cache size must be a positive integer Gi value.'))
      return
    }
    if (publishAfterCreate && !accessGroup.trim()) {
      toast.error(t('A fixed access group is required for publication.'))
      return
    }
    setSubmitting(true)
    const response = await createDeployment({
      ...values,
      codes: source.id.trim() ? [source] : [],
    })
    if (!response.success) {
      setSubmitting(false)
      toast.error(response.message || t('Failed to create deployment'))
      return
    }
    if (publishAfterCreate && response.data?.id) {
      const publication = await publishDeployment(
        values.domain,
        response.data.id,
        {
          access_group: accessGroup.trim(),
          idempotency_key: crypto.randomUUID(),
        }
      )
      if (!publication.success) {
        setSubmitting(false)
        toast.error(
          `${t('Deployment created, but publication did not complete.')}: ${publication.message || t('Operation failed')}`
        )
        void queryClient.invalidateQueries({
          queryKey: deploymentsQueryKeys.lists(),
        })
        onOpenChange(false)
        return
      }
      toast.success(t('Deployment created and published'))
    } else {
      toast.success(t('Deployment created'))
    }
    setSubmitting(false)
    onCreated(values.domain)
    setValues({ ...defaults, domain: values.domain })
    onOpenChange(false)
    void queryClient.invalidateQueries({
      queryKey: deploymentsQueryKeys.lists(),
    })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-3xl'>
        <SheetHeader>
          <SheetTitle>{t('Create deployment')}</SheetTitle>
          <SheetDescription>
            {t(
              'Create a VLLM application in the configured Flyte2 project and selected domain.'
            )}
          </SheetDescription>
        </SheetHeader>
        <form
          id='flyte2-create-form'
          className='grid gap-5 px-4 md:grid-cols-2'
          onSubmit={submit}
        >
          <Field label={t('Deployment domain')} className='md:col-span-2'>
            <Select
              value={values.domain}
              onValueChange={(domain) =>
                set('domain', domain as DeploymentDomain)
              }
            >
              <SelectTrigger className='min-w-48'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {deploymentDomains.map((domain) => (
                    <SelectItem key={domain} value={domain}>
                      {t(domainLabel(domain))}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field label={t('Name')}>
            <Input
              value={values.name}
              onChange={(e) => set('name', e.target.value)}
            />
          </Field>
          <Field label={t('Application ID')}>
            <Input
              value={values.id}
              onChange={(e) => set('id', e.target.value)}
            />
          </Field>
          <Field label={t('Model code')}>
            <Input
              value={values.code}
              onChange={(e) => set('code', e.target.value)}
            />
          </Field>
          <Field label={t('Container image')}>
            <Input
              value={values.image}
              onChange={(e) => set('image', e.target.value)}
            />
          </Field>
          <Field label={t('CPU')}>
            <Input
              value={resource.cpu}
              onChange={(e) => setResource('cpu', e.target.value)}
            />
          </Field>
          <Field label={t('Memory')}>
            <Input
              value={resource.memory}
              onChange={(e) => setResource('memory', e.target.value)}
            />
          </Field>
          <Field label={t('GPU')}>
            <Input
              type='number'
              min={0}
              value={resource.gpu}
              onChange={(e) => setResource('gpu', Number(e.target.value))}
            />
          </Field>
          <Field label={t('Model cache size')}>
            <Input
              value={values.modelCacheSize}
              onChange={(e) => set('modelCacheSize', e.target.value)}
              placeholder='80Gi'
            />
          </Field>
          <Field label={t('GPU resource key')}>
            <Input
              value={resource.gpuResourceKey}
              onChange={(e) => setResource('gpuResourceKey', e.target.value)}
            />
          </Field>
          <Field label={t('GPU node label key')}>
            <Input
              value={resource.gpuNodeLabelKey}
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
          <div className='border-border grid gap-4 rounded-lg border p-4 md:col-span-2 md:grid-cols-2'>
            <h3 className='font-medium md:col-span-2'>{t('Code source')}</h3>
            <Field label={t('Source address')}>
              <Input
                value={source.id}
                onChange={(e) => setSource('id', e.target.value)}
              />
            </Field>
            <Field label={t('Branch')}>
              <Input
                value={source.branch}
                onChange={(e) => setSource('branch', e.target.value)}
              />
            </Field>
            <Field label={t('Path')}>
              <Input
                value={source.path}
                onChange={(e) => setSource('path', e.target.value)}
              />
            </Field>
            <Field label={t('Access token')}>
              <Input
                type='password'
                value={source.token}
                onChange={(e) => setSource('token', e.target.value)}
                autoComplete='new-password'
              />
            </Field>
          </div>
          {publicationEnabled ? (
            <div className='border-border grid gap-4 rounded-lg border p-4 md:col-span-2 md:grid-cols-2'>
              <label className='flex items-center gap-2 md:col-span-2'>
                <Checkbox
                  checked={publishAfterCreate}
                  onCheckedChange={(checked) =>
                    setPublishAfterCreate(checked === true)
                  }
                />
                <span className='text-sm font-medium'>
                  {t('Publish after creation')}
                </span>
              </label>
              {publishAfterCreate ? (
                <Field label={t('Fixed access group')}>
                  <Input
                    value={accessGroup}
                    onChange={(event) => setAccessGroup(event.target.value)}
                  />
                </Field>
              ) : null}
            </div>
          ) : null}
        </form>
        <SheetFooter>
          <Button type='submit' form='flyte2-create-form' disabled={submitting}>
            {submitting ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Create')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function domainLabel(domain: DeploymentDomain) {
  return domain.charAt(0).toUpperCase() + domain.slice(1)
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
