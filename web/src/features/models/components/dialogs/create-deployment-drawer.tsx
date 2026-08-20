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
import { Switch } from '@/components/ui/switch'
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
  type NewPublicationToken,
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
  const [tokenIDs, setTokenIDs] = useState('')
  const [createNewToken, setCreateNewToken] = useState(false)
  const [newToken, setNewToken] =
    useState<NewPublicationToken>(newTokenDefaults)
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
    const parsedTokenIDs = tokenIDs
      .split(',')
      .map((value) => Number(value.trim()))
      .filter((value) => Number.isInteger(value) && value > 0)
    if (
      publishAfterCreate &&
      (!accessGroup.trim() || (parsedTokenIDs.length === 0 && !createNewToken))
    ) {
      toast.error(
        t(
          'A fixed access group and at least one API key ID are required for publication.'
        )
      )
      return
    }
    if (
      publishAfterCreate &&
      createNewToken &&
      (newToken.user_id <= 0 || !newToken.name.trim())
    ) {
      toast.error(t('API key owner user ID and name are required.'))
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
          token_ids: parsedTokenIDs,
          idempotency_key: crypto.randomUUID(),
          new_token: createNewToken
            ? { ...newToken, name: newToken.name.trim() }
            : undefined,
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
                <>
                  <Field label={t('Fixed access group')}>
                    <Input
                      value={accessGroup}
                      onChange={(event) => setAccessGroup(event.target.value)}
                    />
                  </Field>
                  <Field label={t('API key IDs')}>
                    <Input
                      value={tokenIDs}
                      onChange={(event) => setTokenIDs(event.target.value)}
                      placeholder='1, 2'
                    />
                  </Field>
                  <p className='text-muted-foreground text-sm md:col-span-2'>
                    {t(
                      'Only API keys already assigned to this fixed group can be bound. Unrestricted keys in the group can access every published model.'
                    )}
                  </p>
                  <label className='flex items-center gap-2 md:col-span-2'>
                    <Checkbox
                      checked={createNewToken}
                      onCheckedChange={(checked) =>
                        setCreateNewToken(checked === true)
                      }
                    />
                    <span className='text-sm font-medium'>
                      {t('Create and bind a new API key')}
                    </span>
                  </label>
                  {createNewToken ? (
                    <>
                      <Field label={t('Owner user ID')}>
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
                      </Field>
                      <Field label={t('API key name')}>
                        <Input
                          value={newToken.name}
                          onChange={(event) =>
                            setNewToken((current) => ({
                              ...current,
                              name: event.target.value,
                            }))
                          }
                        />
                      </Field>
                      <Field label={t('Expiration timestamp')}>
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
                      </Field>
                      {!newToken.unlimited_quota ? (
                        <Field label={t('Quota')}>
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
                        </Field>
                      ) : (
                        <div />
                      )}
                      <ToggleField
                        label={t('Unlimited Quota')}
                        checked={newToken.unlimited_quota}
                        onCheckedChange={(checked) =>
                          setNewToken((current) => ({
                            ...current,
                            unlimited_quota: checked,
                          }))
                        }
                      />
                      <ToggleField
                        label={t('Restrict key to bound models')}
                        checked={newToken.model_limits_enabled}
                        onCheckedChange={(checked) =>
                          setNewToken((current) => ({
                            ...current,
                            model_limits_enabled: checked,
                          }))
                        }
                      />
                      <ToggleField
                        label={t('Cross-group retry')}
                        checked={newToken.cross_group_retry}
                        onCheckedChange={(checked) =>
                          setNewToken((current) => ({
                            ...current,
                            cross_group_retry: checked,
                          }))
                        }
                      />
                      <Field
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
                      </Field>
                    </>
                  ) : null}
                </>
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

function ToggleField({
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
