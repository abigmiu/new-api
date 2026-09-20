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
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { UpstreamGroups } = await import('../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('upstream groups layout', () => {
  after(() => {
    domWindow.close()
  })

  test('keeps the group table vertically scrollable inside the fixed page content', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    queryClient.setQueryData(['upstream-groups'], {
      success: true,
      data: [],
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <TooltipProvider>
              <UpstreamGroups />
            </TooltipProvider>
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const scrollArea = container.querySelector<HTMLElement>(
      '#upstream-groups-scroll-area'
    )
    assert.ok(scrollArea)
    assert.equal(scrollArea.classList.contains('h-full'), true)
    assert.equal(scrollArea.classList.contains('min-h-0'), true)
    assert.equal(scrollArea.classList.contains('overflow-auto'), true)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })

  test('switches supplier tabs and renders only that supplier groups', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    queryClient.setQueryData(['upstream-groups'], {
      success: true,
      data: [
        {
          id: 1,
          upstream_channel_id: 2,
          upstream_channel_name: 'Supplier',
          upstream_channel_type: 'new-api',
          remote_group_name: 'pro',
          remote_description: 'Premium models',
          local_group: 'uo-2-3',
          local_display_name: '渠道2-pro',
          upstream_key_ready: true,
          desired_enabled: true,
          state: 'active',
          source_ratio: '0.075',
          sale_ratio: '0.089',
          price_version: 1,
          affected_key_count: 0,
          disabled_reason: '',
          last_synced_at: 1,
        },
        {
          id: 2,
          upstream_channel_id: 3,
          upstream_channel_name: 'Supplier 2',
          upstream_channel_type: 'new-api',
          remote_group_name: 'standard',
          remote_description: 'Standard models',
          local_group: 'uo-3-4',
          local_display_name: '渠道3-standard',
          upstream_key_ready: true,
          desired_enabled: true,
          state: 'active',
          source_ratio: '0.1',
          sale_ratio: '0.118',
          price_version: 1,
          affected_key_count: 0,
          disabled_reason: '',
          last_synced_at: 1,
        },
      ],
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <TooltipProvider>
              <UpstreamGroups />
            </TooltipProvider>
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    assert.equal(container.textContent?.includes('Premium models'), true)
    assert.equal(container.textContent?.includes('Standard models'), false)

    const secondSupplierTab = [
      ...container.querySelectorAll<HTMLButtonElement>(
        'button[data-slot="tabs-trigger"]'
      ),
    ].find((button) => button.textContent?.includes('Supplier 2'))
    assert.ok(secondSupplierTab)
    await act(async () => secondSupplierTab.click())
    assert.equal(container.textContent?.includes('Premium models'), false)
    assert.equal(container.textContent?.includes('Standard models'), true)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
