/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useState } from 'react'
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
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import {
  deleteDeployment,
  listDeployments,
  startDeployment,
  stopDeployment,
} from '../api'
import { deploymentsQueryKeys } from '../lib'
import type { Deployment } from '../types'
import {
  deploymentStatusName,
  useDeploymentsColumns,
} from './deployments-columns'
import { UpdateConfigDialog } from './dialogs/update-config-dialog'
import { ViewDetailsDialog } from './dialogs/view-details-dialog'
import { ViewLogsDialog } from './dialogs/view-logs-dialog'

const route = getRouteApi('/_authenticated/models/$section')

export function DeploymentsTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const [dialog, setDialog] = useState<{
    type: 'logs' | 'details' | 'update'
    id: string
  }>()
  const [deleteTarget, setDeleteTarget] = useState<Deployment>()
  const [deleting, setDeleting] = useState(false)
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
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
    onViewLogs: (id) => setDialog({ type: 'logs', id }),
    onViewDetails: (id) => setDialog({ type: 'details', id }),
    onUpdate: (id) => setDialog({ type: 'update', id }),
    onStart: (deployment) =>
      void runAction(() => startDeployment(deployment.id)),
    onStop: (deployment) => void runAction(() => stopDeployment(deployment.id)),
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
    const response = await deleteDeployment(deleteTarget.id)
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
      />
      <ViewDetailsDialog
        open={dialog?.type === 'details'}
        onOpenChange={(open) => !open && setDialog(undefined)}
        deploymentId={dialog?.type === 'details' ? dialog.id : null}
      />
      <UpdateConfigDialog
        open={dialog?.type === 'update'}
        onOpenChange={(open) => !open && setDialog(undefined)}
        deploymentId={dialog?.type === 'update' ? dialog.id : null}
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
