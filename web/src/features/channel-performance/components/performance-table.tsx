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
import dayjs from 'dayjs'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getSuccessRateLevel } from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

import { duration, percent, throughput } from '../lib/format'
import type {
  ChannelPerformanceBucket,
  ChannelPerformanceRow,
  ChannelPerformanceSort,
  ChannelPerformanceSortKey,
  ManagedGroupPerformance,
} from '../types'

type MetricColumn = {
  key: ChannelPerformanceSortKey
  label: string
  cell: (group: ManagedGroupPerformance) => string
}

const METRIC_COLUMNS: MetricColumn[] = [
  {
    key: 'sale_ratio',
    label: 'Price',
    cell: (group) => group.sale_ratio || '—',
  },
  {
    key: 'attempt_count',
    label: 'Requests',
    cell: (group) => group.attempt_count.toLocaleString(),
  },
  {
    key: 'success_rate',
    label: 'Success rate',
    cell: (group) => (group.attempt_count ? percent(group.success_rate) : '—'),
  },
  {
    key: 'avg_latency_ms',
    label: 'Latency',
    cell: (group) => duration(group.avg_latency_ms),
  },
  {
    key: 'avg_ttft_ms',
    label: 'Average TTFT',
    cell: (group) => duration(group.avg_ttft_ms),
  },
  {
    key: 'avg_tps',
    label: 'TPS',
    cell: (group) => throughput(group.avg_tps),
  },
  {
    key: 'cache_hit_rate',
    label: 'Cache hit rate',
    cell: (group) => percent(group.cache_hit_rate),
  },
  {
    key: 'cache_rate',
    label: 'Cache rate',
    cell: (group) => percent(group.cache_rate),
  },
]

export function PerformanceTable(props: {
  rows: ChannelPerformanceRow[]
  showSupplier: boolean
  sort: ChannelPerformanceSort
  onSortChange: (key: ChannelPerformanceSortKey) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto border'>
      <table className='w-full min-w-[1080px] text-sm'>
        <thead className='bg-muted/40 text-muted-foreground'>
          <tr className='border-b text-left'>
            {props.showSupplier ? (
              <SortableHeader
                label={t('Supplier')}
                columnKey='supplier'
                sort={props.sort}
                onSortChange={props.onSortChange}
              />
            ) : null}
            <SortableHeader
              label={t('Group')}
              columnKey='group'
              sort={props.sort}
              onSortChange={props.onSortChange}
            />
            {METRIC_COLUMNS.map((column) => (
              <SortableHeader
                key={column.key}
                label={t(column.label)}
                columnKey={column.key}
                sort={props.sort}
                onSortChange={props.onSortChange}
              />
            ))}
            <th className='w-48 px-3 py-2 font-medium'>{t('Trend')}</th>
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <PerformanceRow
              key={`${row.supplierId}:${row.group.binding_id}`}
              row={row}
              showSupplier={props.showSupplier}
            />
          ))}
        </tbody>
      </table>
    </div>
  )
}

function SortableHeader(props: {
  label: string
  columnKey: ChannelPerformanceSortKey
  sort: ChannelPerformanceSort
  onSortChange: (key: ChannelPerformanceSortKey) => void
}) {
  const { t } = useTranslation()
  const active = props.sort.key === props.columnKey
  let ariaSort: 'none' | 'ascending' | 'descending' = 'none'
  let Icon = ChevronsUpDown
  if (active) {
    ariaSort = props.sort.descending ? 'descending' : 'ascending'
    Icon = props.sort.descending ? ArrowDown : ArrowUp
  }

  return (
    <th aria-sort={ariaSort} className='px-3 py-2 font-medium'>
      <button
        type='button'
        onClick={() => props.onSortChange(props.columnKey)}
        className='focus-visible:ring-ring -mx-1 flex items-center gap-1 rounded-sm px-1 font-medium outline-none focus-visible:ring-2'
        aria-label={t('Sort by {{column}}', { column: props.label })}
      >
        <span>{props.label}</span>
        <Icon
          className={cn('size-3.5', !active && 'text-muted-foreground/60')}
          aria-hidden='true'
        />
      </button>
    </th>
  )
}

function PerformanceRow(props: {
  row: ChannelPerformanceRow
  showSupplier: boolean
}) {
  const { t } = useTranslation()
  const group = props.row.group
  return (
    <tr className='border-b last:border-b-0'>
      {props.showSupplier ? (
        <td className='max-w-60 px-3 py-3 align-top'>
          <div className='font-medium'>{props.row.supplierName}</div>
        </td>
      ) : null}
      <td className='max-w-72 px-3 py-3 align-top'>
        <div className='font-medium'>{group.group_name}</div>
        {group.description ? (
          <div className='text-muted-foreground mt-1 line-clamp-2 text-xs'>
            {group.description}
          </div>
        ) : null}
      </td>
      {METRIC_COLUMNS.map((column) => (
        <MetricCell key={column.key} value={column.cell(group)} />
      ))}
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
