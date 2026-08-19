/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  getDeploymentSettings,
  testDeploymentConnection,
  updateDeploymentSettings,
} from '@/features/models/api'

import { SettingsSection } from '../components/settings-section'

type Values = {
  enabled: boolean
  base_url: string
  project: string
  api_key: string
  configured: boolean
  publication_enabled: boolean
}

const emptyValues: Values = {
  enabled: false,
  base_url: '',
  project: 'aione',
  api_key: '',
  configured: false,
  publication_enabled: false,
}

export function Flyte2DeploymentSettingsSection() {
  const { t } = useTranslation()
  const [values, setValues] = useState<Values>(emptyValues)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{
    ok: boolean
    message: string
  }>()

  useEffect(() => {
    void getDeploymentSettings().then((response) => {
      if (response.success && response.data) {
        setValues({ ...response.data, api_key: '' })
      }
      setLoading(false)
    })
  }, [])

  const setField = <K extends keyof Values>(field: K, value: Values[K]) => {
    setValues((current) => ({ ...current, [field]: value }))
  }

  const save = async () => {
    setSaving(true)
    const response = await updateDeploymentSettings({
      enabled: values.enabled,
      base_url: values.base_url.trim(),
      project: values.project.trim(),
      api_key: values.api_key.trim() || undefined,
      publication_enabled: values.publication_enabled,
    })
    setSaving(false)
    if (!response.success) {
      toast.error(response.message || t('Save failed'))
      return
    }
    setValues((current) => ({
      ...current,
      api_key: '',
      configured: response.data?.configured ?? current.configured,
    }))
    toast.success(t('Saved successfully'))
  }

  const test = async () => {
    setTesting(true)
    setTestResult(undefined)
    const response = await testDeploymentConnection({
      base_url: values.base_url.trim(),
      project: values.project.trim(),
      api_key: values.api_key.trim() || undefined,
    })
    setTesting(false)
    setTestResult({
      ok: response.success,
      message: response.success
        ? t('Connection successful')
        : response.message || t('Connection failed'),
    })
  }

  return (
    <SettingsSection title={t('Flyte2 Deployments')}>
      <div className='space-y-6'>
        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div>
            <Label htmlFor='flyte2-enabled'>
              {t('Enable Flyte2 deployments')}
            </Label>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Manage VLLM applications through the Flyte2 external API.')}
            </p>
          </div>
          <Switch
            id='flyte2-enabled'
            checked={values.enabled}
            onCheckedChange={(checked) => setField('enabled', checked)}
            disabled={loading || saving}
          />
        </div>

        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div>
            <Label htmlFor='flyte2-publication-enabled'>
              {t('Enable Flyte2 model publication')}
            </Label>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t(
                'Allow new deployments and API keys to be bound to the managed Flyte2 channel.'
              )}
            </p>
          </div>
          <Switch
            id='flyte2-publication-enabled'
            checked={values.publication_enabled}
            onCheckedChange={(checked) =>
              setField('publication_enabled', checked)
            }
            disabled={loading || saving || !values.enabled}
          />
        </div>

        <div className='grid gap-4'>
          <Field label={t('Base URL')}>
            <Input
              value={values.base_url}
              onChange={(event) => setField('base_url', event.target.value)}
              placeholder='http://172.19.66.218:30081/v2'
              disabled={loading || saving}
            />
          </Field>
          <Field label={t('Project')}>
            <Input
              value={values.project}
              onChange={(event) => setField('project', event.target.value)}
              disabled={loading || saving}
            />
          </Field>
          <Field label={t('API Key')}>
            <Input
              type='password'
              value={values.api_key}
              onChange={(event) => setField('api_key', event.target.value)}
              placeholder={
                values.configured
                  ? t('Leave blank to keep the saved API key')
                  : t('Enter API Key')
              }
              autoComplete='new-password'
              disabled={loading || saving}
            />
          </Field>
        </div>

        {testResult ? (
          <Alert variant={testResult.ok ? 'default' : 'destructive'}>
            {testResult.ok ? <CheckCircle2 /> : <XCircle />}
            <AlertTitle>{testResult.message}</AlertTitle>
            <AlertDescription>
              {testResult.ok
                ? t('All Flyte2 deployment domains are accessible.')
                : t('Check the URL, API key, and project.')}
            </AlertDescription>
          </Alert>
        ) : null}

        <div className='flex justify-end gap-2'>
          <Button
            variant='outline'
            onClick={test}
            disabled={loading || testing}
          >
            {testing ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Test Connection')}
          </Button>
          <Button onClick={save} disabled={loading || saving}>
            {saving ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Save settings')}
          </Button>
        </div>
      </div>
    </SettingsSection>
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
