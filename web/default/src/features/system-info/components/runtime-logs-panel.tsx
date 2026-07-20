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
import { useInfiniteQuery } from '@tanstack/react-query'
import { FileText, Loader2, RefreshCw, Search } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/design-system/button'
import { Input } from '@/components/design-system/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/design-system/select'
import { ErrorState } from '@/components/error-state'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { getRuntimeLogs } from '../api'
import type { RuntimeLogLine } from '../types'

const LOG_LIMIT = 200
const LOG_SKELETON_KEYS = [
  'runtime-log-skeleton-1',
  'runtime-log-skeleton-2',
  'runtime-log-skeleton-3',
  'runtime-log-skeleton-4',
]

const LEVEL_OPTIONS = ['all', 'INFO', 'WARN', 'ERR', 'FRONTEND_ERROR'] as const

function getLineVariant(line: string) {
  if (line.includes('[FRONTEND_ERROR]') || line.startsWith('[ERR]')) {
    return 'text-destructive'
  }
  if (line.startsWith('[WARN]')) return 'text-warning'
  if (line.startsWith('[INFO]')) return 'text-foreground'
  return 'text-muted-foreground'
}

function RuntimeLogList(props: { lines: RuntimeLogLine[] }) {
  const { t } = useTranslation()

  if (props.lines.length === 0) {
    return (
      <div className='text-muted-foreground px-4 py-10 text-center text-sm'>
        {t('No logs found.')}
      </div>
    )
  }

  return (
    <div className='max-h-[520px] overflow-auto'>
      <div className='min-w-[960px] divide-y'>
        {props.lines.map((item) => (
          <div
            key={item.offset}
            className='grid grid-cols-[72px_1fr] gap-3 px-4 py-2 text-xs'
          >
            <span className='text-muted-foreground tabular-nums'>
              {item.offset + 1}
            </span>
            <pre
              className={cn(
                'font-mono whitespace-pre-wrap break-words',
                getLineVariant(item.line)
              )}
            >
              {item.line}
            </pre>
          </div>
        ))}
      </div>
    </div>
  )
}

export function RuntimeLogsPanel() {
  const { t } = useTranslation()
  const [level, setLevel] = useState<(typeof LEVEL_OPTIONS)[number]>('all')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')

  const logsQuery = useInfiniteQuery({
    queryKey: ['system-info', 'runtime-logs', level, keyword],
    queryFn: async ({ pageParam }) => {
      const res = await getRuntimeLogs({
        cursor: pageParam,
        level: level === 'all' ? undefined : level,
        keyword: keyword || undefined,
        limit: LOG_LIMIT,
      })
      if (!res.success || !res.data) {
        throw new Error(res.message || 'runtime logs unavailable')
      }
      return res.data
    },
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (lastPage) => {
      if (!lastPage.enabled || lastPage.lines.length < LOG_LIMIT)
        return undefined
      return lastPage.lines.at(-1)?.offset
    },
    staleTime: 10 * 1000,
    retry: false,
  })

  const firstPage = logsQuery.data?.pages[0]
  const lines = logsQuery.data?.pages.flatMap((page) => page.lines) ?? []
  const refreshing = logsQuery.isFetching && !logsQuery.isLoading

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setKeyword(keywordInput.trim())
  }

  let content: ReactNode
  if (logsQuery.isLoading) {
    content = (
      <div className='space-y-2 p-4 sm:p-5'>
        {LOG_SKELETON_KEYS.map((key) => (
          <Skeleton key={key} className='h-9 w-full rounded-md' />
        ))}
      </div>
    )
  } else if (logsQuery.isError) {
    content = (
      <ErrorState
        title={t('We could not load runtime logs.')}
        description={
          logsQuery.error instanceof Error ? logsQuery.error.message : undefined
        }
        onRetry={() => {
          void logsQuery.refetch()
        }}
        className='min-h-[260px]'
      />
    )
  } else if (firstPage && !firstPage.enabled) {
    content = (
      <div className='text-muted-foreground px-4 py-10 text-center text-sm'>
        {t('Runtime log file is not enabled.')}
      </div>
    )
  } else {
    content = (
      <>
        <RuntimeLogList lines={lines} />
        <div className='flex items-center justify-between gap-3 border-t px-4 py-3'>
          <span className='text-muted-foreground text-xs tabular-nums'>
            {t('{{count}} lines', { count: lines.length })}
          </span>
          <Button
            type='button'
            variant='outline'
            onClick={() => void logsQuery.fetchNextPage()}
            disabled={!logsQuery.hasNextPage || logsQuery.isFetchingNextPage}
          >
            {logsQuery.isFetchingNextPage && (
              <Loader2
                data-icon='inline-start'
                className='size-3.5 animate-spin'
                aria-hidden='true'
              />
            )}
            {t('Load more')}
          </Button>
        </div>
      </>
    )
  }

  return (
    <section className='bg-card overflow-hidden rounded-lg border shadow-xs'>
      <div className='flex flex-col gap-3 border-b px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5'>
        <div className='min-w-0'>
          <div className='flex items-center gap-2'>
            <span className='bg-muted text-muted-foreground inline-flex size-7 items-center justify-center rounded-md'>
              <FileText className='size-4' aria-hidden='true' />
            </span>
            <h3 className='text-sm font-semibold'>{t('Runtime Logs')}</h3>
          </div>
        </div>
        <form
          className='flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-end'
          onSubmit={handleSubmit}
        >
          <Select
            value={level}
            onValueChange={(value) =>
              setLevel(value as (typeof LEVEL_OPTIONS)[number])
            }
          >
            <SelectTrigger className='w-full sm:w-[160px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LEVEL_OPTIONS.map((option) => (
                <SelectItem key={option} value={option}>
                  {option === 'all' ? t('All levels') : option}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className='flex min-w-0 gap-2'>
            <Input
              value={keywordInput}
              onChange={(event) => setKeywordInput(event.target.value)}
              placeholder={t('Search logs')}
              className='min-w-0 sm:w-[220px]'
              maxLength={120}
            />
            <Button type='submit' variant='outline' aria-label={t('Search')}>
              <Search
                data-icon='inline-start'
                className='size-3.5'
                aria-hidden='true'
              />
              {t('Search')}
            </Button>
            <Button
              type='button'
              variant='outline'
              onClick={() => void logsQuery.refetch()}
              disabled={logsQuery.isFetching}
              aria-label={t('Refresh')}
            >
              <RefreshCw
                data-icon='inline-start'
                className={cn('size-3.5', refreshing && 'animate-spin')}
                aria-hidden='true'
              />
              {refreshing ? t('Refreshing...') : t('Refresh')}
            </Button>
          </div>
        </form>
      </div>
      <div aria-busy={logsQuery.isFetching}>{content}</div>
    </section>
  )
}
