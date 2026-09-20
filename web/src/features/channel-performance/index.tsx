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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getSuccessRateLevel } from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

import { getChannelPerformance } from './api'
import type {
  ChannelPerformanceBucket,
  ChannelPerformanceRange,
  ManagedGroupPerformance,
} from './types'

const ranges: ChannelPerformanceRange[] = ['1h', '24h', '7d']

function percent(value: number | null): string {
  return value == null || !Number.isFinite(value) ? '—' : `${value.toFixed(2)}%`
}

function duration(value: number): string {
  if (!value) return '—'
  if (value < 1000) return `${value.toFixed(0)}ms`
  return `${(value / 1000).toFixed(2)}s`
}

function throughput(value: number): string {
  return value > 0 ? `${value.toFixed(1)} t/s` : '—'
}

export function ChannelPerformance() {
  const { t } = useTranslation()
  const [range, setRange] = useState<ChannelPerformanceRange>('1h')
  const performanceQuery = useQuery({
    queryKey: ['channel-performance', range],
    queryFn: () => getChannelPerformance(range),
    refetchInterval: range === '1h' ? 30_000 : 60_000,
    staleTime: 15_000,
  })
  const suppliers = performanceQuery.data?.data.suppliers ?? []

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
  } else if (suppliers.length === 0) {
    content = (
      <div className='text-muted-foreground border py-12 text-center text-sm'>
        {t('No enabled managed groups')}
      </div>
    )
  } else {
    content = (
      <div className='space-y-6'>
        {suppliers.map((supplier) => (
          <section key={supplier.upstream_channel_id}>
            <div className='mb-2 flex items-baseline gap-2'>
              <h2 className='text-base font-semibold'>
                {supplier.upstream_channel_name}
              </h2>
              <span className='text-muted-foreground text-xs'>
                {t('{{count}} groups', { count: supplier.groups.length })}
              </span>
            </div>
            <div className='overflow-x-auto border'>
              <table className='w-full min-w-[1080px] text-sm'>
                <thead className='bg-muted/40 text-muted-foreground'>
                  <tr className='border-b text-left'>
                    <th className='px-3 py-2 font-medium'>{t('Group')}</th>
                    <th className='px-3 py-2 font-medium'>{t('Price')}</th>
                    <th className='px-3 py-2 font-medium'>{t('Requests')}</th>
                    <th className='px-3 py-2 font-medium'>
                      {t('Success rate')}
                    </th>
                    <th className='px-3 py-2 font-medium'>{t('Latency')}</th>
                    <th className='px-3 py-2 font-medium'>
                      {t('Average TTFT')}
                    </th>
                    <th className='px-3 py-2 font-medium'>TPS</th>
                    <th className='px-3 py-2 font-medium'>
                      {t('Cache hit rate')}
                    </th>
                    <th className='px-3 py-2 font-medium'>{t('Cache rate')}</th>
                    <th className='w-48 px-3 py-2 font-medium'>{t('Trend')}</th>
                  </tr>
                </thead>
                <tbody>
                  {supplier.groups.map((group) => (
                    <GroupPerformanceRow key={group.binding_id} group={group} />
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        ))}
      </div>
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
          {content}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function GroupPerformanceRow(props: { group: ManagedGroupPerformance }) {
  const { t } = useTranslation()
  const group = props.group
  return (
    <tr className='border-b last:border-b-0'>
      <td className='max-w-72 px-3 py-3 align-top'>
        <div className='font-medium'>{group.group_name}</div>
        {group.description ? (
          <div className='text-muted-foreground mt-1 line-clamp-2 text-xs'>
            {group.description}
          </div>
        ) : null}
      </td>
      <MetricCell value={group.sale_ratio || '—'} />
      <MetricCell value={group.attempt_count.toLocaleString()} />
      <MetricCell
        value={group.attempt_count ? percent(group.success_rate) : '—'}
      />
      <MetricCell value={duration(group.avg_latency_ms)} />
      <MetricCell value={duration(group.avg_ttft_ms)} />
      <MetricCell value={throughput(group.avg_tps)} />
      <MetricCell value={percent(group.cache_hit_rate)} />
      <MetricCell value={percent(group.cache_rate)} />
      <td className='px-3 py-3'>
        {group.series.length > 0 ? (
          <PerformanceBars series={group.series} />
        ) : (
          <span className='text-muted-foreground text-xs'>{t('No data')}</span>
        )}
      </td>
    </tr>
  )
}

function MetricCell(props: { value: string }) {
  return (
    <td className='px-3 py-3 align-top font-mono whitespace-nowrap tabular-nums'>
      {props.value}
    </td>
  )
}

function PerformanceBars(props: { series: ChannelPerformanceBucket[] }) {
  const { t } = useTranslation()
  return (
    <div
      className='flex h-9 items-end gap-0.5'
      aria-label={t('Success rate trend')}
    >
      {props.series.map((bucket) => {
        const hasData = bucket.attempt_count > 0
        const height = hasData ? Math.max(8, bucket.success_rate) : 8
        const colorClass = hasData
          ? {
              excellent: 'bg-emerald-500',
              good: 'bg-emerald-400',
              warning: 'bg-amber-500',
              critical: 'bg-red-500',
              unknown: 'bg-muted',
            }[getSuccessRateLevel(bucket.success_rate)]
          : 'bg-muted'
        return (
          <Tooltip key={bucket.start_ts}>
            <TooltipTrigger
              render={
                <button
                  type='button'
                  className={cn(
                    'focus-visible:ring-ring min-w-0 flex-1 rounded-sm outline-none focus-visible:ring-2',
                    colorClass
                  )}
                  style={{ height: `${height}%` }}
                  aria-label={`${dayjs.unix(bucket.start_ts).format('MM-DD HH:mm')} · ${percent(bucket.success_rate)}`}
                />
              }
            />
            <TooltipContent>
              <span>{dayjs.unix(bucket.start_ts).format('MM-DD HH:mm')}</span>
              <br />
              <span>
                {t('Requests')}: {bucket.attempt_count}
              </span>
              <br />
              <span>
                {t('Success rate')}: {percent(bucket.success_rate)}
              </span>
              <br />
              <span>
                {t('Average latency')}:{' '}
                {duration(
                  bucket.total_latency_ms && bucket.attempt_count
                    ? bucket.total_latency_ms / bucket.attempt_count
                    : 0
                )}
              </span>
              <br />
              <span>
                {t('Average TTFT')}: {duration(bucket.avg_ttft_ms)}
              </span>
              <br />
              <span>TPS: {throughput(bucket.avg_tps)}</span>
            </TooltipContent>
          </Tooltip>
        )
      })}
    </div>
  )
}
