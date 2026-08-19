/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import {
  deleteDeployment,
  listDeployments,
  startDeployment,
  stopDeployment,
} from '../api'
import { deploymentsQueryKeys } from '../lib'
import {
  DEPLOYMENT_DOMAINS,
  type Deployment,
  type DeploymentDomain,
} from '../types'
import {
  deploymentStatusName,
  useDeploymentsColumns,
} from './deployments-columns'
import { PublicationDialog } from './dialogs/publication-dialog'
import { UpdateConfigDialog } from './dialogs/update-config-dialog'
import { ViewDetailsDialog } from './dialogs/view-details-dialog'
import { ViewLogsDialog } from './dialogs/view-logs-dialog'

const route = getRouteApi('/_authenticated/models/$section')

export function DeploymentsTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [dialog, setDialog] = useState<{
    type: 'logs' | 'details' | 'update' | 'publication'
    id: string
    domain: DeploymentDomain
  }>()
  const [deleteTarget, setDeleteTarget] = useState<Deployment>()
  const [deleting, setDeleting] = useState(false)
  const routeSearch = route.useSearch()
  const navigate = route.useNavigate()
  const domain = routeSearch.dDomain ?? 'development'
  useEffect(() => {
    if (routeSearch.deployment) {
      setDialog({
        type: 'publication',
        id: routeSearch.deployment,
        domain,
      })
    }
  }, [domain, routeSearch.deployment])
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: routeSearch,
    navigate: route.useNavigate(),
    pagination: {
      pageKey: 'dPage',
      pageSizeKey: 'dPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 8 : 10,
    },
    globalFilter: { enabled: true, key: 'dFilter' },
    columnFilters: [
      { columnId: 'deploymentStatus', searchKey: 'dStatus', type: 'array' },
    ],
  })
  const statusValues =
    (columnFilters.find((filter) => filter.id === 'deploymentStatus')
      ?.value as string[]) || []
  const status = statusValues.includes('all') ? undefined : statusValues[0]
  const params = {
    domain,
    keyword: globalFilter || undefined,
    status,
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
  }
  const { data, isLoading, isFetching } = useQuery({
    queryKey: deploymentsQueryKeys.list(params),
    queryFn: () => listDeployments(params),
    placeholderData: (previous) => previous,
    refetchInterval: () =>
      document.visibilityState === 'visible' ? 10_000 : false,
  })

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: deploymentsQueryKeys.lists() })
  const runAction = async (
    action: () => Promise<{ success: boolean; message?: string }>
  ) => {
    const response = await action()
    if (!response.success) {
      toast.error(response.message || t('Operation failed'))
      return
    }
    toast.success(t('Operation successful'))
    void refresh()
  }
  const columns = useDeploymentsColumns({
    onViewLogs: (deployment) =>
      setDialog({ type: 'logs', id: deployment.id, domain: deployment.domain }),
    onViewDetails: (deployment) =>
      setDialog({
        type: 'details',
        id: deployment.id,
        domain: deployment.domain,
      }),
    onUpdate: (deployment) =>
      setDialog({
        type: 'update',
        id: deployment.id,
        domain: deployment.domain,
      }),
    onPublication: (deployment) =>
      setDialog({
        type: 'publication',
        id: deployment.id,
        domain: deployment.domain,
      }),
    onStart: (deployment) =>
      void runAction(() => startDeployment(deployment.domain, deployment.id)),
    onStop: (deployment) =>
      void runAction(() => stopDeployment(deployment.domain, deployment.id)),
    onDelete: setDeleteTarget,
  })
  const deployments = data?.data?.items || []
  const { table } = useDataTable({
    data: deployments,
    columns,
    totalCount: data?.data?.total || 0,
    columnFilters,
    pagination,
    globalFilter,
    onColumnFiltersChange,
    onPaginationChange,
    onGlobalFilterChange,
    manualPagination: true,
    manualFiltering: true,
    withSortedRowModel: false,
    ensurePageInRange,
  })

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    const response = await deleteDeployment(
      deleteTarget.domain,
      deleteTarget.id
    )
    setDeleting(false)
    if (response.success) {
      toast.success(t('Deleted successfully'))
      setDeleteTarget(undefined)
      void refresh()
    } else {
      toast.error(response.message || t('Delete failed'))
    }
  }

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Deployments Found')}
        emptyDescription={t(
          'No deployments available. Create one to get started.'
        )}
        skeletonKeyPrefix='deployment-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search deployments...'),
          searchDebounceMs: 500,
          additionalSearch: (
            <Select
              value={domain}
              onValueChange={(value) => {
                setDialog(undefined)
                void navigate({
                  search: (previous) => ({
                    ...previous,
                    dDomain: value as DeploymentDomain,
                    dPage: 1,
                    deployment: undefined,
                  }),
                })
              }}
            >
              <SelectTrigger
                className='min-w-40'
                aria-label={t('Deployment domain')}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {DEPLOYMENT_DOMAINS.map((value) => (
                    <SelectItem key={value} value={value}>
                      {t(domainLabel(value))}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          ),
          filters: [
            {
              columnId: 'deploymentStatus',
              title: t('Status'),
              singleSelect: true,
              options: [
                { label: t('All Status'), value: 'all' },
                ...[1, 2, 3, 4, 6, 7, 8, 9, 10].map((value) => ({
                  label: t(deploymentStatusName(value)),
                  value: String(value),
                })),
              ],
            },
          ],
        }}
      />
      <ViewLogsDialog
        open={dialog?.type === 'logs'}
        onOpenChange={(open) => !open && setDialog(undefined)}
        deploymentId={dialog?.type === 'logs' ? dialog.id : null}
        domain={dialog?.type === 'logs' ? dialog.domain : domain}
      />
      <ViewDetailsDialog
        open={dialog?.type === 'details'}
        onOpenChange={(open) => !open && setDialog(undefined)}
        deploymentId={dialog?.type === 'details' ? dialog.id : null}
        domain={dialog?.type === 'details' ? dialog.domain : domain}
      />
      <UpdateConfigDialog
        open={dialog?.type === 'update'}
        onOpenChange={(open) => !open && setDialog(undefined)}
        deploymentId={dialog?.type === 'update' ? dialog.id : null}
        domain={dialog?.type === 'update' ? dialog.domain : domain}
      />
      <PublicationDialog
        open={dialog?.type === 'publication'}
        onOpenChange={(open) => !open && setDialog(undefined)}
        deploymentId={dialog?.type === 'publication' ? dialog.id : null}
        domain={dialog?.type === 'publication' ? dialog.domain : domain}
      />
      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(undefined)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm delete')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Are you sure you want to delete deployment "{{name}}"? This action cannot be undone.',
                { name: deleteTarget?.name }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={confirmDelete}
              disabled={deleting}
            >
              {deleting ? t('Deleting...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function domainLabel(domain: DeploymentDomain) {
  return domain.charAt(0).toUpperCase() + domain.slice(1)
}
