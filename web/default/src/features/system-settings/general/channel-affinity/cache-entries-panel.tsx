import type { ColumnDef, PaginationState } from '@tanstack/react-table'
import { Search, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DataTablePagination,
  DataTableView,
  useDataTable,
} from '@/components/data-table'
import { Button } from '@/components/design-system/button'
import { Input } from '@/components/design-system/input'

import { deleteCacheEntry, getCacheEntries, updateCacheEntry } from './api'
import type { CacheEntry } from './types'

const PAGE_SIZE = 20

type Props = {
  onChanged: () => void
}

export function CacheEntriesPanel(props: Props) {
  const { t } = useTranslation()
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [data, setData] = useState<CacheEntry[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [updatingID, setUpdatingID] = useState<string | null>(null)
  const [channelIDs, setChannelIDs] = useState<Record<string, string>>({})
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: PAGE_SIZE,
  })

  const loadEntries = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getCacheEntries({
        page: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        keyword: keyword || undefined,
      })
      if (!res.success || !res.data) {
        toast.error(res.message || t('Request failed'))
        return
      }
      setData(res.data.items)
      setTotal(res.data.total)
      setChannelIDs(
        Object.fromEntries(
          res.data.items.map((entry) => [entry.id, String(entry.channel_id)])
        )
      )
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setLoading(false)
    }
  }, [keyword, pagination, t])

  useEffect(() => {
    void loadEntries()
  }, [loadEntries])

  const columns = useMemo<ColumnDef<CacheEntry, unknown>[]>(
    () => [
      { accessorKey: 'rule_name', header: t('Rule'), size: 150 },
      {
        id: 'scope',
        header: t('Model / Group'),
        size: 220,
        cell: ({ row }) =>
          [row.original.model_name, row.original.using_group]
            .filter(Boolean)
            .join(' / ') || '-',
      },
      { accessorKey: 'key_hint', header: t('Key Summary'), size: 160 },
      { accessorKey: 'key_fp', header: t('Key Fingerprint'), size: 130 },
      {
        id: 'channel',
        header: t('Channel ID'),
        size: 180,
        cell: ({ row }) => (
          <Input
            type='number'
            min={1}
            value={channelIDs[row.original.id] || ''}
            onChange={(event) =>
              setChannelIDs((current) => ({
                ...current,
                [row.original.id]: event.target.value,
              }))
            }
          />
        ),
      },
      {
        id: 'actions',
        header: t('Actions'),
        size: 130,
        cell: ({ row }) => {
          const entry = row.original
          const channelID = Number(channelIDs[entry.id])
          const isUpdating = updatingID === entry.id
          return (
            <div className='flex gap-1'>
              <Button
                size='sm'
                disabled={isUpdating || !Number.isInteger(channelID) || channelID < 1}
                onClick={async () => {
                  setUpdatingID(entry.id)
                  try {
                    const res = await updateCacheEntry(entry.id, channelID)
                    if (!res.success) {
                      toast.error(res.message || t('Request failed'))
                      return
                    }
                    toast.success(t('Saved successfully'))
                    props.onChanged()
                    void loadEntries()
                  } finally {
                    setUpdatingID(null)
                  }
                }}
              >
                {t('Switch')}
              </Button>
              <Button
                variant='ghost'
                size='icon-sm'
                aria-label={t('Delete')}
                disabled={isUpdating}
                onClick={async () => {
                  setUpdatingID(entry.id)
                  try {
                    const res = await deleteCacheEntry(entry.id)
                    if (!res.success) {
                      toast.error(res.message || t('Request failed'))
                      return
                    }
                    toast.success(t('Deleted successfully'))
                    props.onChanged()
                    void loadEntries()
                  } finally {
                    setUpdatingID(null)
                  }
                }}
              >
                <Trash2 className='h-3 w-3' />
              </Button>
            </div>
          )
        },
      },
    ],
    [channelIDs, loadEntries, props, t, updatingID]
  )

  const { table } = useDataTable({
    data,
    columns,
    pagination,
    onPaginationChange: setPagination,
    manualPagination: true,
    pageCount: Math.ceil(total / pagination.pageSize),
    withSortedRowModel: false,
    withFacetedRowModel: false,
  })

  return (
    <div className='flex min-h-[420px] flex-col gap-4'>
      <form
        className='flex shrink-0 gap-2'
        onSubmit={(event) => {
          event.preventDefault()
          setPagination((current) => ({ ...current, pageIndex: 0 }))
          setKeyword(keywordInput.trim())
        }}
      >
        <div className='relative flex-1'>
          <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
          <Input
            placeholder={t('Search cache entries...')}
            value={keywordInput}
            onChange={(event) => setKeywordInput(event.target.value)}
            className='ps-9'
          />
        </div>
        <Button type='submit' variant='outline'>
          {t('Search')}
        </Button>
      </form>
      <DataTableView
        table={table}
        containerClassName='min-h-0 flex-1 rounded-md'
        tableContainerClassName='h-full min-h-0'
        splitHeaderScrollClassName='h-full'
        splitHeader
        isLoading={loading}
        emptyContent={t('No cache entries found')}
      />
      <div className='shrink-0'>
        <DataTablePagination table={table} />
      </div>
    </div>
  )
}
