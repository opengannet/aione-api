/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useCallback, useEffect, useState } from 'react'

import { getDeploymentSettings, testDeploymentConnection } from '../api'
import type { DeploymentSettingsResponse } from '../types'

type LoadingPhase = 'idle' | 'settings' | 'connection' | 'done'

export function useModelDeploymentSettings() {
  const [settings, setSettings] = useState<
    DeploymentSettingsResponse['data'] | undefined
  >()
  const [loading, setLoading] = useState(true)
  const [loadingPhase, setLoadingPhase] = useState<LoadingPhase>('settings')
  const [connectionLoading, setConnectionLoading] = useState(false)
  const [connectionOk, setConnectionOk] = useState<boolean | null>(null)
  const [connectionError, setConnectionError] = useState<string | null>(null)

  const testConnection = useCallback(async () => {
    setLoadingPhase('connection')
    setConnectionLoading(true)
    const response = await testDeploymentConnection()
    setConnectionLoading(false)
    setLoadingPhase('done')
    setConnectionOk(response.success)
    setConnectionError(
      response.success ? null : response.message || 'Connection failed'
    )
  }, [])

  const refresh = useCallback(async () => {
    setLoading(true)
    setLoadingPhase('settings')
    const response = await getDeploymentSettings()
    const nextSettings = response.success ? response.data : undefined
    setSettings(nextSettings)
    setLoading(false)
    if (nextSettings?.enabled) {
      await testConnection()
    } else {
      setLoadingPhase('done')
      setConnectionOk(null)
      setConnectionError(null)
    }
  }, [testConnection])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return {
    loading,
    loadingPhase,
    settings,
    isFlyte2Enabled: settings?.enabled === true,
    refresh,
    connectionLoading,
    connectionOk,
    connectionError,
    testConnection,
  }
}
