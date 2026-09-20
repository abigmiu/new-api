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

function group(overrides: {
  binding_id: number
  group_name: string
  attempt_count: number
  success_rate: number
  avg_latency_ms: number
}) {
  return {
    description: '',
    sale_ratio: '0.100',
    success_count: 0,
    avg_ttft_ms: 0,
    avg_tps: 0,
    cache_hit_rate: null,
    cache_rate: null,
    series: [],
    ...overrides,
  }
}

// Primary holds one healthy group and one group that never served a request,
// Secondary holds a single group, so every sort direction can be told apart.
const performanceFixture = {
  success: true,
  data: {
    updated_at: 1_754_890_123,
    range: '1h',
    suppliers: [
      {
        upstream_channel_id: 1,
        upstream_channel_name: 'Primary',
        groups: [
          group({
            binding_id: 1,
            group_name: 'Alpha',
            attempt_count: 10,
            success_rate: 90,
            avg_latency_ms: 2000,
          }),
          group({
            binding_id: 2,
            group_name: 'Beta',
            attempt_count: 5,
            success_rate: 20,
            avg_latency_ms: 0,
          }),
        ],
      },
      {
        upstream_channel_id: 2,
        upstream_channel_name: 'Secondary',
        groups: [
          group({
            binding_id: 3,
            group_name: 'Gamma',
            attempt_count: 20,
            success_rate: 70,
            avg_latency_ms: 1000,
          }),
        ],
      },
    ],
  },
}

async function mount() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(['channel-performance', '1h'], performanceFixture)

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
  return { container, root, queryClient }
}

async function unmount(mounted: Awaited<ReturnType<typeof mount>>) {
  await act(async () => mounted.root.unmount())
  mounted.queryClient.clear()
  mounted.container.remove()
}

// The group name lives in the first cell of a supplier table and in the
// second cell of the "All" table, which prepends a supplier column.
function renderedColumn(container: HTMLElement, index: number): string[] {
  return [...container.querySelectorAll('tbody tr')].map(
    (row) => row.children[index]?.textContent?.trim() ?? ''
  )
}

function sortedHeaderAttribute(container: HTMLElement): {
  label: string
  ariaSort: string | null
} {
  const header = container.querySelector(
    'th[aria-sort="ascending"], th[aria-sort="descending"]'
  )
  assert.ok(header, 'no column is marked as sorted')
  return {
    label: header.textContent ?? '',
    ariaSort: header.getAttribute('aria-sort'),
  }
}

function tabNamed(container: HTMLElement, name: string) {
  return [
    ...container.querySelectorAll<HTMLButtonElement>(
      'button[data-slot="tabs-trigger"]'
    ),
  ].find((button) => button.textContent?.includes(name))
}

async function clickSort(container: HTMLElement, label: string) {
  const header = container.querySelector<HTMLButtonElement>(
    `button[aria-label="Sort by ${label}"]`
  )
  assert.ok(header, `missing sortable header for ${label}`)
  await act(async () => header.click())
}

describe('channel performance all tab and sorting', () => {
  after(() => {
    domWindow.close()
  })

  test('opens on the All tab, sorted by the best success rate, with a supplier column', async () => {
    const mounted = await mount()

    const activeTab = mounted.container.querySelector(
      'button[data-slot="tabs-trigger"][aria-selected="true"]'
    )
    assert.ok(activeTab)
    assert.match(activeTab.textContent ?? '', /All/)

    const headers = [...mounted.container.querySelectorAll('th')].map(
      (header) => header.textContent ?? ''
    )
    assert.ok(headers.some((header) => header.includes('Supplier')))

    assert.equal(
      mounted.container.textContent?.includes('3 groups'),
      true,
      'the All tab should count the groups of every supplier'
    )
    const defaultSort = sortedHeaderAttribute(mounted.container)
    assert.match(defaultSort.label, /Success rate/)
    assert.equal(defaultSort.ariaSort, 'descending')
    assert.deepEqual(renderedColumn(mounted.container, 1), [
      'Alpha',
      'Gamma',
      'Beta',
    ])
    assert.deepEqual(renderedColumn(mounted.container, 0), [
      'Primary',
      'Secondary',
      'Primary',
    ])

    await unmount(mounted)
  })

  test('sorts the shared table by the clicked column in both directions', async () => {
    const mounted = await mount()

    await clickSort(mounted.container, 'Requests')
    assert.deepEqual(renderedColumn(mounted.container, 1), [
      'Beta',
      'Alpha',
      'Gamma',
    ])
    const ascending = sortedHeaderAttribute(mounted.container)
    assert.match(ascending.label, /Requests/)
    assert.equal(ascending.ariaSort, 'ascending')

    await clickSort(mounted.container, 'Requests')
    assert.deepEqual(renderedColumn(mounted.container, 1), [
      'Gamma',
      'Alpha',
      'Beta',
    ])
    const descending = sortedHeaderAttribute(mounted.container)
    assert.match(descending.label, /Requests/)
    assert.equal(descending.ariaSort, 'descending')

    await unmount(mounted)
  })

  test('keeps groups without measurements last in both sort directions', async () => {
    const mounted = await mount()

    await clickSort(mounted.container, 'Latency')
    assert.deepEqual(renderedColumn(mounted.container, 1), [
      'Gamma',
      'Alpha',
      'Beta',
    ])

    await clickSort(mounted.container, 'Latency')
    assert.deepEqual(renderedColumn(mounted.container, 1), [
      'Alpha',
      'Gamma',
      'Beta',
    ])

    await unmount(mounted)
  })

  test('narrows to one supplier and drops the supplier column when its tab is selected', async () => {
    const mounted = await mount()

    const primaryTab = tabNamed(mounted.container, 'Primary')
    assert.ok(primaryTab)
    await act(async () => primaryTab.click())

    const headers = [...mounted.container.querySelectorAll('th')].map(
      (header) => header.textContent ?? ''
    )
    assert.equal(
      headers.some((header) => header.includes('Supplier')),
      false
    )
    assert.equal(mounted.container.textContent?.includes('2 groups'), true)
    assert.deepEqual(renderedColumn(mounted.container, 0), ['Alpha', 'Beta'])

    await unmount(mounted)
  })
})
