/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { RefreshCw } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { getChannelPerformance } from './api'
import { PerformanceTable } from './components/performance-table'
import {
  DEFAULT_CHANNEL_PERFORMANCE_SORT,
  sortPerformanceRows,
} from './lib/sort'
import type {
  ChannelPerformanceRange,
  ChannelPerformanceRow,
  ChannelPerformanceSortKey,
} from './types'

const ranges: ChannelPerformanceRange[] = ['1h', '24h', '7d']

const ALL_SUPPLIERS = 'all'

export function ChannelPerformance() {
  const { t } = useTranslation()
  const [range, setRange] = useState<ChannelPerformanceRange>('1h')
  const [selectedTab, setSelectedTab] = useState<string>(ALL_SUPPLIERS)
  const [sort, setSort] = useState(DEFAULT_CHANNEL_PERFORMANCE_SORT)
  const performanceQuery = useQuery({
    queryKey: ['channel-performance', range],
    queryFn: () => getChannelPerformance(range),
    refetchInterval: range === '1h' ? 30_000 : 60_000,
    staleTime: 15_000,
  })
  const suppliers = useMemo(
    () => performanceQuery.data?.data.suppliers ?? [],
    [performanceQuery.data]
  )
  const showAll = selectedTab === ALL_SUPPLIERS
  const visibleSupplier = suppliers.find(
    (supplier) => String(supplier.upstream_channel_id) === selectedTab
  )
  const rows = useMemo<ChannelPerformanceRow[]>(() => {
    const all = suppliers.flatMap((supplier) =>
      supplier.groups.map((group) => ({
        supplierId: supplier.upstream_channel_id,
        supplierName: supplier.upstream_channel_name,
        group,
      }))
    )
    if (showAll) return all
    return all.filter((row) => String(row.supplierId) === selectedTab)
  }, [suppliers, showAll, selectedTab])
  const sortedRows = useMemo(
    () => sortPerformanceRows(rows, sort),
    [rows, sort]
  )
  const toggleSort = (key: ChannelPerformanceSortKey) =>
    setSort((current) =>
      current.key === key
        ? { key, descending: !current.descending }
        : { key, descending: false }
    )

  let content: ReactNode
  if (performanceQuery.isError) {
    content = (
      <div className='text-destructive border py-12 text-center text-sm'>
        {t('Failed to load channel performance')}
      </div>
    )
  } else if (performanceQuery.isLoading) {
    content = (
      <div className='space-y-4'>
        {[0, 1].map((key) => (
          <Skeleton key={key} className='h-72 rounded-lg' />
        ))}
      </div>
    )
  } else if (suppliers.length === 0 || (!showAll && !visibleSupplier)) {
    content = (
      <div className='text-muted-foreground border py-12 text-center text-sm'>
        {t('No enabled managed groups')}
      </div>
    )
  } else {
    content = (
      <section>
        <div className='mb-2 flex items-baseline gap-2'>
          <h2 className='text-base font-semibold'>
            {showAll
              ? t('All')
              : (visibleSupplier?.upstream_channel_name ?? '')}
          </h2>
          <span className='text-muted-foreground text-xs'>
            {t('{{count}} groups', { count: sortedRows.length })}
          </span>
        </div>
        <PerformanceTable
          rows={sortedRows}
          showSupplier={showAll}
          sort={sort}
          onSortChange={toggleSort}
        />
      </section>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Channel Performance')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center rounded-lg border p-0.5' role='group'>
          {ranges.map((value) => (
            <Button
              key={value}
              type='button'
              size='sm'
              variant={range === value ? 'secondary' : 'ghost'}
              aria-pressed={range === value}
              onClick={() => setRange(value)}
            >
              {value}
            </Button>
          ))}
        </div>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                type='button'
                size='icon'
                variant='outline'
                aria-label={t('Refresh')}
                onClick={() => performanceQuery.refetch()}
                disabled={performanceQuery.isFetching}
              />
            }
          >
            <RefreshCw
              className={cn(performanceQuery.isFetching && 'animate-spin')}
            />
          </TooltipTrigger>
          <TooltipContent>{t('Refresh')}</TooltipContent>
        </Tooltip>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='text-muted-foreground flex min-h-4 justify-end text-xs'>
            {performanceQuery.data?.data.updated_at ? (
              <span>
                {t('Updated at {{time}}', {
                  time: dayjs
                    .unix(performanceQuery.data.data.updated_at)
                    .format('HH:mm:ss'),
                })}
              </span>
            ) : null}
          </div>
          {suppliers.length > 0 ? (
            <Tabs
              value={selectedTab}
              onValueChange={(value) => value !== null && setSelectedTab(value)}
              className='overflow-x-auto'
            >
              <TabsList>
                <TabsTrigger value={ALL_SUPPLIERS}>{t('All')}</TabsTrigger>
                {suppliers.map((supplier) => (
                  <TabsTrigger
                    key={supplier.upstream_channel_id}
                    value={String(supplier.upstream_channel_id)}
                  >
                    {supplier.upstream_channel_name}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          ) : null}
          {content}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
