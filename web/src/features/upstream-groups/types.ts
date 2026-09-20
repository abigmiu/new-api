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
export type UpstreamGroup = {
  id: number
  upstream_channel_id: number
  upstream_channel_name: string
  upstream_channel_type: string
  remote_group_name: string
  remote_description: string
  local_group: string
  local_display_name: string
  local_channel_id?: number | null
  upstream_key_ready: boolean
  desired_enabled: boolean
  state: string
  source_ratio: string
  sale_ratio: string
  price_version: number
  affected_key_count: number
  disabled_reason: string
  last_synced_at: number
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}
