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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Route, Save } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/design-system/button'
import { Combobox } from '@/components/design-system/combobox'
import { SectionPageLayout } from '@/components/layout'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'

import { getChannelPreferences, updateChannelPreferences } from './api'

const QUERY_KEY = ['channel-preferences'] as const

export function ChannelPreferences() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [preferences, setPreferences] = useState<Record<string, string>>({})
  const query = useQuery({
    queryKey: QUERY_KEY,
    queryFn: async () => {
      const response = await getChannelPreferences()
      if (!response.success) {
        throw new Error(
          response.message || t('Failed to load channel preferences')
        )
      }
      return response.data?.groups ?? []
    },
  })

  useEffect(() => {
    if (!query.data) return
    setPreferences(
      Object.fromEntries(
        query.data.map((group) => [group.group, group.selected_alias])
      )
    )
  }, [query.data])

  const mutation = useMutation({
    mutationFn: updateChannelPreferences,
    onSuccess: async (response) => {
      if (!response.success) {
        toast.error(response.message || t('Failed to save channel preferences'))
        return
      }
      toast.success(t('Channel preferences saved'))
      await queryClient.invalidateQueries({ queryKey: QUERY_KEY })
    },
    onError: () => toast.error(t('Failed to save channel preferences')),
  })

  const groups = query.data ?? []
  let content: ReactNode

  if (query.isLoading) {
    content = (
      <div className='divide-border rounded-lg border px-4 sm:px-5'>
        {[0, 1, 2].map((index) => (
          <div
            key={index}
            className='flex min-h-16 flex-col justify-center gap-2 border-b py-3 last:border-b-0 sm:flex-row sm:items-center sm:justify-between'
          >
            <Skeleton className='h-4 w-28' />
            <Skeleton className='h-7 w-full sm:h-8 sm:w-72' />
          </div>
        ))}
      </div>
    )
  } else if (groups.length === 0) {
    content = (
      <Empty className='min-h-64 border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Route aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('No available groups')}</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = (
      <form
        onSubmit={(event) => {
          event.preventDefault()
          mutation.mutate({ preferences })
        }}
        className='overflow-hidden rounded-lg border'
      >
        <div className='divide-border divide-y px-4 sm:px-5'>
          {groups.map((group) => {
            const options = [
              { value: '', label: t('Follow system') },
              ...group.channels.map((channel) => ({
                value: channel.alias,
                label: channel.display_name,
              })),
            ]
            return (
              <div
                key={group.group}
                className='grid min-h-16 gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_18rem] sm:items-center sm:gap-6'
              >
                <Label htmlFor={`channel-preference-${group.group}`}>
                  {group.group}
                </Label>
                <Combobox
                  id={`channel-preference-${group.group}`}
                  options={options}
                  value={preferences[group.group] ?? ''}
                  onValueChange={(value) =>
                    setPreferences((current) => ({
                      ...current,
                      [group.group]: value ?? '',
                    }))
                  }
                  placeholder={t('Select channel')}
                  searchPlaceholder={t('Search channels...')}
                  emptyText={t('No channels found')}
                  openOnFocus={false}
                />
              </div>
            )
          })}
        </div>
        <div className='bg-muted/30 flex justify-end border-t px-4 py-3 sm:px-5'>
          <Button type='submit' disabled={mutation.isPending}>
            <Save aria-hidden='true' />
            {t('Save')}
          </Button>
        </div>
      </form>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Channel Preferences')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-4xl'>{content}</div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
