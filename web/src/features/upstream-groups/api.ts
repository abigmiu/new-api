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
import { api } from '@/lib/api'

import type { ApiResponse, UpstreamGroup } from './types'

export async function getUpstreamGroups(): Promise<
  ApiResponse<UpstreamGroup[]>
> {
  const response = await api.get('/api/upstream-groups')
  return response.data
}

export async function syncUpstreamGroups(): Promise<ApiResponse> {
  const response = await api.post('/api/upstream-groups/sync')
  return response.data
}

export async function setUpstreamGroupEnabled(
  id: number,
  enabled: boolean
): Promise<ApiResponse<UpstreamGroup>> {
  const action = enabled ? 'enable' : 'disable'
  const response = await api.post(`/api/upstream-groups/${id}/${action}`)
  return response.data
}
