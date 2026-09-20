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
import { Check, PowerOff } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'

import { acceptTokenGroupPrice, disableTokenGroup } from '../api'
import type { ApiKey } from '../types'

type ManagedTokenGroupsProps = {
  tokenId: number
  groups: NonNullable<ApiKey['groups']>
  onChanged: () => Promise<unknown>
}

const STATUS_VARIANTS = {
  active: 'success',
  price_changed: 'warning',
  group_unavailable: 'danger',
  user_disabled: 'neutral',
} as const

const STATUS_LABELS = {
  active: 'Active',
  price_changed: 'Price changed',
  group_unavailable: 'Unavailable',
  user_disabled: 'Disabled for this key',
} as const

export function ManagedTokenGroups(props: ManagedTokenGroupsProps) {
  const { t } = useTranslation()
  const [pendingBindingId, setPendingBindingId] = useState<number | null>(null)

  const updateGroup = async (
    bindingId: number,
    action: () => Promise<{ success: boolean; message?: string }>
  ) => {
    setPendingBindingId(bindingId)
    try {
      const result = await action()
      if (!result.success) {
        toast.error(result.message || t('Failed to update managed group'))
        return
      }
      await props.onChanged()
      toast.success(t('Managed group updated'))
    } catch {
      toast.error(t('Failed to update managed group'))
    } finally {
      setPendingBindingId(null)
    }
  }

  return (
    <div className='space-y-2'>
      {props.groups.map((group) => (
        <div
          key={group.binding_id}
          className='flex min-w-0 items-center gap-3 rounded-md border px-3 py-2'
        >
          <div className='min-w-0 flex-1'>
            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <span className='truncate text-sm font-medium'>
                {group.display_name}
              </span>
              <StatusBadge
                label={t(STATUS_LABELS[group.state])}
                variant={STATUS_VARIANTS[group.state]}
                copyable={false}
              />
            </div>
            <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
              {t('Accepted price')}: {group.accepted_sale_ratio}
              {group.state === 'price_changed' && (
                <span>
                  {' -> '} {t('Current price')}: {group.current_sale_ratio}
                </span>
              )}
            </div>
          </div>
          {(group.state === 'price_changed' ||
            group.state === 'user_disabled') && (
            <Button
              type='button'
              size='sm'
              disabled={pendingBindingId === group.binding_id}
              onClick={() =>
                updateGroup(group.binding_id, () =>
                  acceptTokenGroupPrice(
                    props.tokenId,
                    group.binding_id,
                    group.current_price_version
                  )
                )
              }
            >
              <Check />
              {group.state === 'price_changed'
                ? t('Accept price')
                : t('Enable for this key')}
            </Button>
          )}
          {group.state === 'active' && (
            <Button
              type='button'
              size='icon-sm'
              variant='ghost'
              aria-label={t('Disable for this key')}
              disabled={pendingBindingId === group.binding_id}
              onClick={() =>
                updateGroup(group.binding_id, () =>
                  disableTokenGroup(props.tokenId, group.binding_id)
                )
              }
            >
              <PowerOff />
            </Button>
          )}
        </div>
      ))}
    </div>
  )
}
