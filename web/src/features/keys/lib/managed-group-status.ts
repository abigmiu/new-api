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
import type { ApiKey } from '../types'

export type ManagedGroupStatusVariant =
  | 'success'
  | 'warning'
  | 'danger'
  | 'neutral'

export type ManagedGroupStatus = {
  /** English source string; callers pass it through `t()`. */
  label: string
  variant: ManagedGroupStatusVariant
}

type ManagedGroup = NonNullable<ApiKey['groups']>[number]

/**
 * Maps a managed group binding to its badge.
 *
 * The backend deliberately collapses every non-actionable reason (upstream key
 * rejected, admin disabled, still provisioning, a leftover multi-binding key)
 * into `group_unavailable`; only a group that genuinely disappeared upstream
 * gets its own wording. Keeping that distinction here — and nowhere else —
 * means no surface can accidentally spell out more of the operator's internal
 * state than the API is willing to share.
 */
export function managedGroupStatus(group: ManagedGroup): ManagedGroupStatus {
  if (group.state === 'active') {
    return { label: 'Active', variant: 'success' }
  }
  if (group.state === 'price_changed') {
    return { label: 'Price changed', variant: 'warning' }
  }
  if (group.state === 'user_disabled') {
    return { label: 'Disabled for this key', variant: 'neutral' }
  }
  if (group.global_state === 'group_removed') {
    return { label: 'Channel removed', variant: 'danger' }
  }
  return { label: 'Unavailable', variant: 'danger' }
}
