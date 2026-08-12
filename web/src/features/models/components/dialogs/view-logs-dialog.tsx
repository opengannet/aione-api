/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import { getDeploymentLogs } from '../../api'

const pageSize = 200

export function ViewLogsDialog({
  open,
  onOpenChange,
  deploymentId,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  deploymentId: string | number | null
}) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['deployment-logs', deploymentId, page],
    queryFn: () => {
      if (deploymentId === null) throw new Error('deployment ID is required')
      return getDeploymentLogs(deploymentId, { page, size: pageSize })
    },
    enabled: open && deploymentId !== null,
    refetchInterval:
      open && document.visibilityState === 'visible' ? 10_000 : false,
  })
  const logs = data?.data?.items || []
  const total = data?.data?.total || 0
  let logContent = (
    <p className='text-muted-foreground py-12 text-center'>
      {t('No logs available')}
    </p>
  )
  if (isLoading) {
    logContent = <Loader2 className='mx-auto my-12 size-8 animate-spin' />
  } else if (logs.length > 0) {
    logContent = (
      <div>
        {logs.map((line) => (
          <div
            key={`${line.timestamp}-${line.message}`}
            className='break-words whitespace-pre-wrap'
          >
            <span className='text-muted-foreground me-3'>{line.timestamp}</span>
            {line.message}
          </div>
        ))}
      </div>
    )
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) setPage(1)
      }}
    >
      <DialogContent className='sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{t('Deployment logs')}</DialogTitle>
          <DialogDescription>{deploymentId}</DialogDescription>
        </DialogHeader>
        <div className='bg-muted/50 h-[60vh] overflow-auto rounded-lg border p-3 font-mono text-xs'>
          {logContent}
        </div>
        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground text-sm'>
            {t('{{total}} log lines', { total })}
            {isFetching ? '…' : ''}
          </span>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              size='icon-sm'
              onClick={() => setPage((value) => Math.max(1, value - 1))}
              disabled={page === 1}
            >
              <ChevronLeft />
            </Button>
            <span className='min-w-10 text-center text-sm leading-8'>
              {page}
            </span>
            <Button
              variant='outline'
              size='icon-sm'
              onClick={() => setPage((value) => value + 1)}
              disabled={page * pageSize >= total}
            >
              <ChevronRight />
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
