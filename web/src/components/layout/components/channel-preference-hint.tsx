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
import { Link } from '@tanstack/react-router'
import { Lightbulb } from 'lucide-react'
import { Trans } from 'react-i18next'

import { cn } from '@/lib/utils'

type ChannelPreferenceHintProps = {
  className?: string
}

export function ChannelPreferenceHint(props: ChannelPreferenceHintProps) {
  return (
    <span
      className={cn(
        'border-warning/40 bg-warning/10 text-warning inline-flex items-start gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium',
        props.className
      )}
    >
      <Lightbulb className='mt-px size-3.5 shrink-0' aria-hidden='true' />
      <span className='line-clamp-2'>
        <Trans
          i18nKey='Check <1>Channel Performance</1> before choosing managed groups'
          components={{
            1: (
              <Link
                to='/performance'
                className='hover:text-warning font-semibold underline underline-offset-2 transition-colors'
              />
            ),
          }}
        />
      </span>
    </span>
  )
}
