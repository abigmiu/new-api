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
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { ApiKeyGroupCell } = await import('../api-key-group-cell')
const { apiKeySchema } = await import('../../types')

type ApiKey = import('../../types').ApiKey
type ManagedGroup = NonNullable<ApiKey['groups']>[number]

function parsedManagedGroup(group: Record<string, unknown>): ManagedGroup {
  const parsed = apiKeySchema.safeParse({
    id: 1,
    name: 'managed',
    key: 'sk-managed',
    status: 1,
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 1,
    accessed_time: 0,
    model_limits_enabled: false,
    groups: [group],
  })
  assert.equal(parsed.success, true)
  if (!parsed.success) throw new Error('managed key fixture is invalid')
  const groups = parsed.data.groups ?? []
  assert.equal(groups.length, 1)
  return groups[0]
}

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Auto: 'Auto',
        'Cross-group': 'Cross-group',
        Ratio: 'Ratio',
        Active: 'Active',
        'Price changed': 'Price changed',
        'Disabled for this key': 'Disabled for this key',
        'Channel removed': 'Channel removed',
        Unavailable: 'Unavailable',
        'Automatically selects the best available group with circuit breaker mechanism':
          'Automatically selects the best available group with circuit breaker mechanism',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function CellHarness(props: {
  group: string
  ratio?: number | string
  crossGroupRetry?: boolean
  shouldReduceMotion?: boolean
  groups?: NonNullable<ApiKey['groups']>
}) {
  return (
    <I18nextProvider i18n={i18n}>
      <TooltipProvider>
        <ApiKeyGroupCell
          group={props.group}
          ratio={props.ratio}
          crossGroupRetry={props.crossGroupRetry ?? false}
          shouldReduceMotion={props.shouldReduceMotion ?? false}
          groups={props.groups}
        />
      </TooltipProvider>
    </I18nextProvider>
  )
}

describe('API key group table cell', () => {
  after(() => {
    domWindow.close()
  })

  test('renders two unclipped rings and a localized Auto ratio when API data uses a nonlocalized string', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness
          group='auto'
          ratio='自动'
          crossGroupRetry
          shouldReduceMotion={false}
        />
      )
    )

    const badgeCell = container.querySelector<HTMLElement>(
      '[data-api-key-group-cell="auto"]'
    )
    assert.ok(badgeCell)
    assert.equal(badgeCell.classList.contains('overflow-visible'), true)
    assert.equal(badgeCell.classList.contains('overflow-hidden'), false)

    const frames = container.querySelectorAll('[data-auto-group-frame]')
    const movingRings = container.querySelectorAll(
      '[data-auto-group-flow-border]'
    )
    assert.equal(frames.length, 2)
    assert.equal(movingRings.length, 2)
    for (const frame of frames) {
      assert.equal(frame.classList.contains('relative'), true)
      assert.equal(frame.classList.contains('overflow-visible'), true)
      assert.equal(frame.classList.contains('rounded-4xl'), true)
      assert.equal(frame.classList.contains('p-px'), true)
    }

    const ratio = container.querySelector<HTMLElement>(
      '[data-auto-group-effect="ratio"]'
    )
    assert.ok(ratio)
    assert.equal(ratio.textContent, 'Auto Ratio')
    assert.equal(ratio.textContent?.includes('x'), false)
    assert.equal(container.textContent?.includes('自动'), false)
    assert.equal(container.textContent?.includes('Cross-group'), true)

    const crossGroupBadge = [
      ...container.querySelectorAll<HTMLElement>('[data-slot="status-badge"]'),
    ].find((badge) => badge.textContent === 'Cross-group')
    assert.ok(crossGroupBadge)
    assert.equal(crossGroupBadge.closest('[data-auto-group-frame]'), null)

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps static Auto frames but omits both moving layers for reduced motion', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(<CellHarness group='auto' ratio='Auto' shouldReduceMotion />)
    )

    assert.equal(
      container.querySelectorAll('[data-auto-group-frame]').length,
      2
    )
    assert.equal(
      container.querySelectorAll('[data-auto-group-flow-border]').length,
      0
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows only the Auto badge when ratio data is unavailable', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(<CellHarness group='auto' shouldReduceMotion={false} />)
    )

    assert.equal(
      container.querySelectorAll('[data-auto-group-frame]').length,
      1
    )
    assert.equal(
      container.querySelectorAll('[data-auto-group-flow-border]').length,
      1
    )
    assert.equal(
      container.querySelector('[data-auto-group-effect="ratio"]'),
      null
    )
    assert.equal(container.textContent?.includes('Auto'), true)
    assert.equal(container.textContent?.includes('Ratio'), false)

    await act(async () => root.unmount())
    container.remove()
  })

  test('renders a single managed badge from the binding state and never a channel id', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    const renderGroups = async (group: ManagedGroup) => {
      await act(async () =>
        root.render(<CellHarness group='' groups={[group]} />)
      )
      return container.textContent ?? ''
    }

    assert.equal(
      await renderGroups(
        parsedManagedGroup({
          binding_id: 1,
          local_group: 'uo-7-x',
          display_name: 'Managed 7',
          position: 1,
          enabled: true,
          effective_enabled: true,
          state: 'active',
          accepted_price_version: 1,
          current_price_version: 1,
          accepted_source_ratio: '1',
          accepted_sale_ratio: '1.15',
          current_source_ratio: '1',
          current_sale_ratio: '1.15',
          global_state: 'active',
          local_channel_id: 7,
        })
      ),
      'Active'
    )

    assert.equal(
      await renderGroups(
        parsedManagedGroup({
          binding_id: 1,
          local_group: 'uo-7-x',
          display_name: 'Managed 7',
          position: 1,
          enabled: false,
          effective_enabled: false,
          state: 'group_unavailable',
          accepted_price_version: 2,
          current_price_version: 2,
          accepted_source_ratio: '1',
          accepted_sale_ratio: '1.15',
          current_source_ratio: '2',
          current_sale_ratio: '2.3',
          global_state: 'group_removed',
          local_channel_id: 7,
        })
      ),
      'Channel removed'
    )

    // The upstream key being rejected, the admin disabling the channel and a
    // leftover multi-binding key all read the same way, so the badge never
    // reveals which internal reason applies.
    for (const globalState of [
      'key_unavailable',
      'admin_disabled',
      'provisioning',
    ]) {
      assert.equal(
        await renderGroups(
          parsedManagedGroup({
            binding_id: 1,
            local_group: 'uo-7-x',
            display_name: 'Managed 7',
            position: 1,
            enabled: true,
            effective_enabled: false,
            state: 'group_unavailable',
            accepted_price_version: 2,
            current_price_version: 2,
            accepted_source_ratio: '1',
            accepted_sale_ratio: '1.15',
            current_source_ratio: '2',
            current_sale_ratio: '2.3',
            global_state: globalState,
            local_channel_id: 7,
          })
        ),
        'Unavailable'
      )
    }

    assert.equal(
      await renderGroups(
        parsedManagedGroup({
          binding_id: 1,
          local_group: 'uo-7-x',
          display_name: 'Managed 7',
          position: 1,
          enabled: true,
          effective_enabled: false,
          state: 'multiple_bindings',
          accepted_price_version: 2,
          current_price_version: 2,
          accepted_source_ratio: '1',
          accepted_sale_ratio: '1.15',
          current_source_ratio: '2',
          current_sale_ratio: '2.3',
          global_state: 'active',
          local_channel_id: 7,
        })
      ),
      'Unavailable'
    )

    const text = container.textContent ?? ''
    assert.equal(text.includes('Managed 7'), false)
    assert.equal(text.includes('uo-7-x'), false)

    await act(async () => root.unmount())
    container.remove()
  })

  test('narrows normal group ratios to numbers and never applies Auto rings', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness group='vip' ratio='自动' shouldReduceMotion={false} />
      )
    )

    assert.equal(container.textContent?.includes('vip'), true)
    assert.equal(container.textContent?.includes('自动'), false)
    assert.equal(container.querySelector('[data-auto-group-frame]'), null)
    assert.equal(container.querySelector('[data-auto-group-flow-border]'), null)

    await act(async () =>
      root.render(
        <CellHarness group='vip' ratio={3} shouldReduceMotion={false} />
      )
    )

    assert.equal(container.textContent?.includes('3x'), true)
    assert.equal(container.querySelector('[data-auto-group-frame]'), null)

    await act(async () => root.unmount())
    container.remove()
  })
})
