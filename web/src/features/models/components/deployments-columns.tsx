/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { ColumnDef } from '@tanstack/react-table'
import {
  FileText,
  Info,
  KeyRound,
  Pencil,
  Play,
  Square,
  Trash2,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'

import type { Deployment } from '../types'

const statusNames: Record<number, string> = {
  0: 'Unspecified',
  1: 'Unassigned',
  2: 'Assigned',
  3: 'Pending',
  4: 'Stopped',
  5: 'Started',
  6: 'Failed',
  7: 'Active',
  8: 'Scaling up',
  9: 'Scaling down',
  10: 'Deploying',
}

export const deploymentStatusName = (status: number) =>
  statusNames[status] || 'Unknown'

function deploymentStatusVariant(status: number) {
  if (status === 7) return 'success' as const
  if (status === 6) return 'danger' as const
  if (status === 10) return 'warning' as const
  return 'neutral' as const
}

export function useDeploymentsColumns(opts: {
  onViewLogs: (deployment: Deployment) => void
  onViewDetails: (deployment: Deployment) => void
  onUpdate: (deployment: Deployment) => void
  onPublication: (deployment: Deployment) => void
  onStart: (deployment: Deployment) => void
  onStop: (deployment: Deployment) => void
  onDelete: (deployment: Deployment) => void
}): ColumnDef<Deployment>[] {
  const { t } = useTranslation()
  return [
    {
      accessorKey: 'deploymentStatus',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.original.deploymentStatus
        return (
          <StatusBadge
            label={t(deploymentStatusName(status))}
            variant={deploymentStatusVariant(status)}
            size='sm'
            copyable={false}
          />
        )
      },
    },
    {
      accessorKey: 'currentReplicas',
      header: t('Replicas'),
      cell: ({ row }) => (
        <span className='font-mono'>{row.original.currentReplicas}</span>
      ),
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <div>
          <div className='font-medium'>{row.original.name}</div>
          <div className='text-muted-foreground font-mono text-xs'>
            {row.original.id}
          </div>
        </div>
      ),
    },
    {
      accessorKey: 'type',
      header: t('Type'),
      cell: ({ row }) => (
        <StatusBadge
          label={row.original.type}
          variant='neutral'
          size='sm'
          copyable={false}
        />
      ),
    },
    {
      accessorKey: 'createdAt',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-sm whitespace-nowrap'>
          {row.original.createdAt
            ? new Date(row.original.createdAt).toLocaleString()
            : '-'}
        </span>
      ),
    },
    {
      id: 'publication',
      header: t('Publication'),
      cell: ({ row }) => {
        let variant: 'success' | 'warning' | 'neutral' = 'neutral'
        if (row.original.publication?.phase === 'published') {
          variant = 'success'
        } else if (row.original.publication) {
          variant = 'warning'
        }
        return (
          <StatusBadge
            label={t(row.original.publication?.phase || 'Not published')}
            variant={variant}
            size='sm'
            copyable={false}
          />
        )
      },
    },
    {
      id: 'actions',
      header: t('Actions'),
      enableSorting: false,
      cell: ({ row }) => {
        const deployment = row.original
        const canStart =
          deployment.deploymentStatus === 1 || deployment.deploymentStatus === 4
        const canDelete = [1, 4, 6].includes(deployment.deploymentStatus)
        let stateAction: ReactNode = null
        if (canStart) {
          stateAction = (
            <DropdownMenuItem onClick={() => opts.onStart(deployment)}>
              {t('Start')}
              <DropdownMenuShortcut>
                <Play size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )
        } else if (deployment.deploymentStatus !== 6) {
          stateAction = (
            <DropdownMenuItem onClick={() => opts.onStop(deployment)}>
              {t('Stop')}
              <DropdownMenuShortcut>
                <Square size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          )
        }
        return (
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => opts.onViewLogs(deployment)}
              aria-label={t('View logs')}
            >
              <FileText />
            </Button>
            <DataTableRowActionMenu ariaLabel={t('Open menu')}>
              <DropdownMenuItem onClick={() => opts.onViewDetails(deployment)}>
                {t('View details')}
                <DropdownMenuShortcut>
                  <Info size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => opts.onUpdate(deployment)}>
                {t('Edit')}
                <DropdownMenuShortcut>
                  <Pencil size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => opts.onPublication(deployment)}>
                {t('Publication and API keys')}
                <DropdownMenuShortcut>
                  <KeyRound size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
              {stateAction}
              {canDelete ? (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className='text-destructive focus:text-destructive'
                    onClick={() => opts.onDelete(deployment)}
                  >
                    {t('Delete')}
                    <DropdownMenuShortcut>
                      <Trash2 size={16} />
                    </DropdownMenuShortcut>
                  </DropdownMenuItem>
                </>
              ) : null}
            </DataTableRowActionMenu>
          </div>
        )
      },
      meta: { pinned: 'right' as const },
    },
  ]
}
