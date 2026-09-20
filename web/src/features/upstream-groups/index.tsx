/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Power, PowerOff, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatTimestampToDate } from '@/lib/format'

import {
  getUpstreamGroups,
  setUpstreamGroupEnabled,
  syncUpstreamGroups,
} from './api'
import type { UpstreamGroup } from './types'

const STATE_VARIANTS: Record<
  string,
  'success' | 'warning' | 'danger' | 'neutral'
> = {
  active: 'success',
  discovered: 'neutral',
  provisioning: 'warning',
  admin_disabled: 'neutral',
  group_removed: 'danger',
  key_unavailable: 'danger',
  error: 'danger',
}

const STATE_LABELS: Record<string, string> = {
  active: 'Active',
  discovered: 'Discovered',
  provisioning: 'Provisioning',
  admin_disabled: 'Globally disabled',
  group_removed: 'Group removed',
  key_unavailable: 'Key unavailable',
  error: 'Error',
}

export function UpstreamGroups() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [pendingChange, setPendingChange] = useState<{
    group: UpstreamGroup
    enabled: boolean
  } | null>(null)
  const [selectedSupplierId, setSelectedSupplierId] = useState<number | null>(
    null
  )
  const groupsQuery = useQuery({
    queryKey: ['upstream-groups'],
    queryFn: getUpstreamGroups,
    refetchInterval: 60_000,
  })
  const syncMutation = useMutation({
    mutationFn: syncUpstreamGroups,
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to start synchronization'))
        return
      }
      toast.success(t('Synchronization started'))
      await queryClient.invalidateQueries({ queryKey: ['upstream-groups'] })
    },
  })
  const stateMutation = useMutation({
    mutationFn: (input: { id: number; enabled: boolean }) =>
      setUpstreamGroupEnabled(input.id, input.enabled),
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to update upstream group'))
        return
      }
      toast.success(t('Upstream group updated'))
      setPendingChange(null)
      await queryClient.invalidateQueries({ queryKey: ['upstream-groups'] })
    },
  })

  const columns = useMemo<StaticDataTableColumn<UpstreamGroup>[]>(
    () => [
      {
        id: 'group',
        header: t('Group'),
        className: 'min-w-52',
        cell: (group) => group.local_display_name,
      },
      {
        id: 'description',
        header: t('Description'),
        className: 'min-w-64',
        cell: (group) => group.remote_description || '-',
      },
      {
        id: 'source',
        header: t('Source price'),
        cellClassName: 'font-mono tabular-nums',
        cell: (group) => group.source_ratio,
      },
      {
        id: 'sale',
        header: t('Sale price'),
        cellClassName: 'font-mono tabular-nums',
        cell: (group) => group.sale_ratio,
      },
      {
        id: 'version',
        header: t('Price version'),
        cell: (group) => `v${group.price_version}`,
      },
      {
        id: 'state',
        header: t('Status'),
        cell: (group) => (
          <StatusBadge
            label={t(STATE_LABELS[group.state] || 'Error')}
            variant={STATE_VARIANTS[group.state] || 'neutral'}
            copyable={false}
          />
        ),
      },
      {
        id: 'key',
        header: t('Supplier key'),
        cell: (group) =>
          group.upstream_key_ready ? t('Ready') : t('Not created'),
      },
      {
        id: 'affected',
        header: t('Affected keys'),
        cellClassName: 'tabular-nums',
        cell: (group) => group.affected_key_count,
      },
      {
        id: 'channel',
        header: t('Local channel'),
        cell: (group) => group.local_channel_id ?? '-',
      },
      {
        id: 'synced',
        header: t('Last synchronized'),
        className: 'min-w-44',
        cell: (group) =>
          group.last_synced_at > 0
            ? formatTimestampToDate(group.last_synced_at)
            : '-',
      },
      {
        id: 'actions',
        header: '',
        cellClassName: 'text-right',
        cell: (group) => {
          const enabled = group.desired_enabled && group.state === 'active'
          return (
            <Button
              size='sm'
              variant={enabled ? 'destructive' : 'outline'}
              disabled={stateMutation.isPending}
              onClick={() => setPendingChange({ group, enabled: !enabled })}
            >
              {enabled ? <PowerOff /> : <Power />}
              {enabled ? t('Disable globally') : t('Enable globally')}
            </Button>
          )
        },
      },
    ],
    [stateMutation, t]
  )

  const groups = useMemo(() => groupsQuery.data?.data || [], [groupsQuery.data])
  const suppliers = useMemo(() => {
    const byId = new Map<number, string>()
    for (const group of groups) {
      byId.set(group.upstream_channel_id, group.upstream_channel_name)
    }
    return [...byId.entries()].map(([id, name]) => ({ id, name }))
  }, [groups])
  const selectedSupplier =
    suppliers.find((supplier) => supplier.id === selectedSupplierId) ??
    suppliers[0]
  const visibleGroups = selectedSupplier
    ? groups.filter(
        (group) => group.upstream_channel_id === selectedSupplier.id
      )
    : []

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Upstream groups')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            size='sm'
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
          >
            <RefreshCw
              className={syncMutation.isPending ? 'animate-spin' : ''}
            />
            {t('Synchronize')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3'>
            {selectedSupplier ? (
              <Tabs
                value={String(selectedSupplier.id)}
                onValueChange={(value) =>
                  value !== null && setSelectedSupplierId(Number(value))
                }
                className='shrink-0 overflow-x-auto'
              >
                <TabsList>
                  {suppliers.map((supplier) => (
                    <TabsTrigger key={supplier.id} value={String(supplier.id)}>
                      {supplier.name}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>
            ) : null}
            <StaticDataTable
              className='h-full min-h-0 overflow-auto **:data-[slot=table-container]:overflow-visible'
              columns={columns}
              containerProps={{ id: 'upstream-groups-scroll-area' }}
              data={visibleGroups}
              getRowKey={(group) => group.id}
              emptyContent={
                groupsQuery.isLoading
                  ? t('Loading...')
                  : t('No upstream groups')
              }
              tableClassName='min-w-[960px]'
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <ConfirmDialog
        open={pendingChange !== null}
        onOpenChange={(open) => !open && setPendingChange(null)}
        title={t('Confirm Action')}
        desc={pendingChange?.group.local_display_name || ''}
        destructive={pendingChange?.enabled === false}
        confirmText={
          pendingChange?.enabled ? t('Confirm enable') : t('Confirm disable')
        }
        isLoading={stateMutation.isPending}
        handleConfirm={() => {
          if (pendingChange) {
            stateMutation.mutate({
              id: pendingChange.group.id,
              enabled: pendingChange.enabled,
            })
          }
        }}
      />
    </>
  )
}
