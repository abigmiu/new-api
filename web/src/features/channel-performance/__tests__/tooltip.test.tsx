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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { ChannelPerformance } = await import('../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('channel performance tooltip', () => {
  after(() => {
    domWindow.close()
  })

  test('shows each bucket metric on its own line when a bar receives focus', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    queryClient.setQueryData(['channel-performance', '1h', 'gpt-0.1倍率'], {
      success: true,
      data: {
        updated_at: 1_754_890_123,
        groups: ['gpt-0.1倍率', 'vip'],
        selected_group: 'gpt-0.1倍率',
        is_admin: true,
        channels: [
          {
            channel_id: 1,
            channel_name: 'Primary',
            display_name: 'Primary',
            alias: 'ABC123',
            channel_type: 1,
            attempt_count: 4,
            success_count: 3,
            success_rate: 75,
            avg_latency_ms: 1000,
            avg_ttft_ms: 300,
            avg_tps: 40,
            cache_hit_rate: null,
            cache_rate: null,
            cache_report_count: 0,
            cache_hit_count: 0,
            cached_input_tokens: 0,
            logical_input_tokens: 0,
            active_model_count: 1,
            series: [
              {
                start_ts: 1_754_890_080,
                end_ts: 1_754_890_200,
                attempt_count: 4,
                success_count: 3,
                total_latency_ms: 4000,
                success_rate: 75,
                avg_ttft_ms: 300,
                avg_tps: 40,
                cache_hit_rate: null,
                cache_rate: null,
              },
            ],
          },
        ],
      },
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <TooltipProvider>
              <ChannelPerformance />
            </TooltipProvider>
          </I18nextProvider>
        </QueryClientProvider>
      )
    })
    const bar = container.querySelector<HTMLButtonElement>(
      'button[aria-label*="75.00%"]'
    )
    assert.ok(bar)
    const groupTrigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Group"]'
    )
    assert.ok(groupTrigger)
    assert.equal(groupTrigger.textContent?.includes('gpt-0.1倍率'), true)
    assert.equal(container.textContent?.includes('Primary'), true)
    assert.equal(container.textContent?.includes('#1 · ABC123'), true)
    await act(async () => groupTrigger.click())
    const vipOption = [
      ...document.body.querySelectorAll('[role="option"]'),
    ].find((option) => option.textContent?.includes('vip'))
    assert.ok(vipOption)
    await act(async () => bar.focus())

    const tooltip = document.body.querySelector('[data-slot="tooltip-content"]')
    assert.ok(tooltip)
    assert.equal(tooltip.querySelectorAll('br').length, 5)
    assert.match(tooltip.textContent ?? '', /Success rate: 75.00%/)
    assert.match(tooltip.textContent ?? '', /Average latency: 1.00s/)
    assert.match(tooltip.textContent ?? '', /Average TTFT: 300ms/)
    assert.match(tooltip.textContent ?? '', /TPS: 40.0 t\/s/)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
