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
import { Activity, Gauge, RefreshCw, Timer } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getChannelTypeLabel } from '@/features/channels/lib/channel-utils'
import { cn } from '@/lib/utils'

import { getChannelPerformance } from './api'
import type {
  ChannelPerformanceBucket,
  ChannelPerformanceItem,
  ChannelPerformanceRange,
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
  const channels = useMemo(
    () => performanceQuery.data?.data.channels ?? [],
    [performanceQuery.data]
  )
  const summary = useMemo(() => {
    let attempts = 0
    let successes = 0
    let cacheReports = 0
    let cacheHits = 0
    let cachedInput = 0
    let logicalInput = 0
    for (const channel of channels) {
      attempts += channel.attempt_count
      successes += channel.success_count
      cacheReports += channel.cache_report_count
      cacheHits += channel.cache_hit_count
      cachedInput += channel.cached_input_tokens
      logicalInput += channel.logical_input_tokens
    }
    return {
      attempts,
      successRate: attempts > 0 ? (successes / attempts) * 100 : null,
      cacheHitRate: cacheReports > 0 ? (cacheHits / cacheReports) * 100 : null,
      cacheRate: logicalInput > 0 ? (cachedInput / logicalInput) * 100 : null,
    }
  }, [channels])

  let channelContent: ReactNode
  if (performanceQuery.isError) {
    channelContent = (
      <div className='text-destructive border py-12 text-center text-sm'>
        {t('Failed to load channel performance')}
      </div>
    )
  } else if (performanceQuery.isLoading) {
    channelContent = (
      <div className='grid grid-cols-1 gap-3 xl:grid-cols-2 2xl:grid-cols-3'>
        {[0, 1, 2].map((key) => (
          <Skeleton key={key} className='h-72 rounded-lg' />
        ))}
      </div>
    )
  } else if (channels.length === 0) {
    channelContent = (
      <div className='text-muted-foreground border py-12 text-center text-sm'>
        {t('No enabled channels')}
      </div>
    )
  } else {
    channelContent = (
      <div className='grid grid-cols-1 gap-3 xl:grid-cols-2 2xl:grid-cols-3'>
        {channels.map((channel) => (
          <ChannelCard key={channel.channel_id} channel={channel} />
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

          <div className='grid border-y sm:grid-cols-2 lg:grid-cols-4 lg:divide-x'>
            <SummaryMetric
              label={t('Requests')}
              value={summary.attempts.toLocaleString()}
            />
            <SummaryMetric
              label={t('Success rate')}
              value={percent(summary.successRate)}
            />
            <SummaryMetric
              label={t('Cache hit rate')}
              value={percent(summary.cacheHitRate)}
            />
            <SummaryMetric
              label={t('Cache rate')}
              value={percent(summary.cacheRate)}
            />
          </div>

          {channelContent}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function SummaryMetric(props: { label: string; value: string }) {
  return (
    <div className='px-4 py-3'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 font-mono text-xl font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function ChannelCard(props: { channel: ChannelPerformanceItem }) {
  const { t } = useTranslation()
  const channel = props.channel
  return (
    <article className='overflow-hidden rounded-lg border'>
      <header className='flex items-start justify-between gap-3 border-b px-4 py-3'>
        <div className='min-w-0'>
          <h3 className='truncate text-sm font-semibold'>
            {channel.channel_name}
          </h3>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            #{channel.channel_id} ·{' '}
            {t(getChannelTypeLabel(channel.channel_type))}
          </p>
        </div>
        <span className='text-muted-foreground shrink-0 text-xs'>
          {t('{{count}} active models', { count: channel.active_model_count })}
        </span>
      </header>

      <div className='grid grid-cols-3 divide-x border-b'>
        <CardMetric
          icon={Timer}
          label={t('Latency')}
          value={duration(channel.avg_latency_ms)}
        />
        <CardMetric
          icon={Activity}
          label={t('Average TTFT')}
          value={duration(channel.avg_ttft_ms)}
        />
        <CardMetric
          icon={Gauge}
          label='TPS'
          value={throughput(channel.avg_tps)}
        />
      </div>

      <div className='bg-muted/20 grid grid-cols-3 border-b px-4 py-3'>
        <CardValue
          label={t('Success rate')}
          value={
            channel.attempt_count > 0 ? percent(channel.success_rate) : '—'
          }
        />
        <CardValue
          label={t('Cache hit rate')}
          value={percent(channel.cache_hit_rate)}
        />
        <CardValue
          label={t('Cache rate')}
          value={percent(channel.cache_rate)}
        />
      </div>

      <div className='px-4 pt-4 pb-3'>
        <PerformanceBars series={channel.series} />
        <div className='text-muted-foreground mt-2 flex justify-between text-[11px]'>
          <span>{t('Past')}</span>
          <span>{t('Now')}</span>
        </div>
      </div>
    </article>
  )
}

function CardMetric(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
}) {
  const Icon = props.icon
  return (
    <div className='min-w-0 px-3 py-3'>
      <div className='text-muted-foreground flex items-center gap-1 text-[11px]'>
        <Icon className='size-3' />
        <span className='truncate'>{props.label}</span>
      </div>
      <div className='mt-1 truncate font-mono text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function CardValue(props: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground truncate text-[11px]'>
        {props.label}
      </div>
      <div className='mt-1 truncate font-mono text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function PerformanceBars(props: { series: ChannelPerformanceBucket[] }) {
  const { t } = useTranslation()
  return (
    <div
      className='flex h-12 items-end gap-1'
      aria-label={t('Success rate trend')}
    >
      {props.series.map((bucket) => {
        const hasData = bucket.attempt_count > 0
        const height = hasData ? Math.max(8, bucket.success_rate) : 8
        return (
          <Tooltip key={bucket.start_ts}>
            <TooltipTrigger
              render={
                <button
                  type='button'
                  className={cn(
                    'focus-visible:ring-ring min-w-0 flex-1 rounded-sm outline-none focus-visible:ring-2',
                    hasData ? 'bg-emerald-500' : 'bg-muted'
                  )}
                  style={{ height: `${height}%` }}
                  aria-label={`${dayjs.unix(bucket.start_ts).format('MM-DD HH:mm')} ${hasData ? percent(bucket.success_rate) : t('No data')}`}
                />
              }
            />
            <TooltipContent className='space-y-1'>
              <div className='font-medium'>
                {dayjs.unix(bucket.start_ts).format('MM-DD HH:mm')} –{' '}
                {dayjs.unix(bucket.end_ts).format('HH:mm')}
              </div>
              <div className='font-mono text-sm'>
                {hasData ? percent(bucket.success_rate) : '—'}
              </div>
              <div className='text-muted-foreground'>
                {t('{{count}} requests', { count: bucket.attempt_count })} ·{' '}
                {duration(bucket.total_latency_ms)}
              </div>
            </TooltipContent>
          </Tooltip>
        )
      })}
    </div>
  )
}
