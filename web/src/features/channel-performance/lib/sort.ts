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
import type {
  ChannelPerformanceRow,
  ChannelPerformanceSort,
  ChannelPerformanceSortKey,
} from '../types'

// A sortable value is either a comparable scalar or `null` when the table
// renders the metric as an em dash (no samples, unknown cache rate, ...).
// Missing values always sink to the bottom, in both sort directions.
type SortValue = number | string | null

// The healthiest groups come first, so the page opens on the best performers.
export const DEFAULT_CHANNEL_PERFORMANCE_SORT: ChannelPerformanceSort = {
  key: 'success_rate',
  descending: true,
}

function numberOrNull(value: number | null): number | null {
  return value == null || !Number.isFinite(value) ? null : value
}

// Latency, TTFT and TPS are rendered as an em dash when they are zero or
// negative, so they must not outrank real measurements while sorting.
function positiveOrNull(value: number): number | null {
  return value > 0 ? numberOrNull(value) : null
}

function sortValueOf(
  row: ChannelPerformanceRow,
  key: ChannelPerformanceSortKey
): SortValue {
  const group = row.group
  switch (key) {
    case 'supplier':
      return row.supplierName.trim() || null
    case 'group':
      return group.group_name.trim() || null
    case 'sale_ratio':
      return numberOrNull(Number.parseFloat(group.sale_ratio))
    case 'attempt_count':
      return group.attempt_count
    case 'success_rate':
      return group.attempt_count > 0 ? numberOrNull(group.success_rate) : null
    case 'avg_latency_ms':
      return positiveOrNull(group.avg_latency_ms)
    case 'avg_ttft_ms':
      return positiveOrNull(group.avg_ttft_ms)
    case 'avg_tps':
      return positiveOrNull(group.avg_tps)
    case 'cache_hit_rate':
      return numberOrNull(group.cache_hit_rate)
    case 'cache_rate':
      return numberOrNull(group.cache_rate)
  }
}

function compareValues(
  left: SortValue,
  right: SortValue,
  descending: boolean
): number {
  if (left === null || right === null) {
    if (left === right) return 0
    return left === null ? 1 : -1
  }
  const result =
    typeof left === 'string' && typeof right === 'string'
      ? left.localeCompare(right)
      : Number(left) - Number(right)
  return descending ? -result : result
}

function compareRows(
  left: ChannelPerformanceRow,
  right: ChannelPerformanceRow
): number {
  const supplier =
    left.supplierName.localeCompare(right.supplierName) ||
    left.supplierId - right.supplierId
  if (supplier !== 0) return supplier
  return (
    left.group.group_name.localeCompare(right.group.group_name) ||
    left.group.binding_id - right.group.binding_id
  )
}

export function sortPerformanceRows(
  rows: ChannelPerformanceRow[],
  sort: ChannelPerformanceSort
): ChannelPerformanceRow[] {
  return [...rows].sort((left, right) => {
    const result = compareValues(
      sortValueOf(left, sort.key),
      sortValueOf(right, sort.key),
      sort.descending
    )
    if (result !== 0) return result
    return compareRows(left, right)
  })
}
