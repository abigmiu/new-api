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
export type ChannelPerformanceRange = '1h' | '24h' | '7d'

export type ChannelPerformanceBucket = {
  start_ts: number
  end_ts: number
  attempt_count: number
  success_count: number
  total_latency_ms: number
  success_rate: number
  avg_ttft_ms: number
  avg_tps: number
  cache_hit_rate: number | null
  cache_rate: number | null
}

export type ManagedGroupPerformance = {
  binding_id: number
  group_name: string
  description: string
  sale_ratio: string
  attempt_count: number
  success_count: number
  success_rate: number
  avg_latency_ms: number
  avg_ttft_ms: number
  avg_tps: number
  cache_hit_rate: number | null
  cache_rate: number | null
  series: ChannelPerformanceBucket[]
}

export type ChannelPerformanceSupplier = {
  upstream_channel_id: number
  upstream_channel_name: string
  groups: ManagedGroupPerformance[]
}

export type ChannelPerformanceResponse = {
  success: boolean
  message?: string
  data: {
    updated_at: number
    range: ChannelPerformanceRange
    suppliers: ChannelPerformanceSupplier[]
  }
}
