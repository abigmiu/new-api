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
type FrontendErrorPayload = {
  type: string
  message: string
  stack?: string
  url?: string
  user_agent?: string
  source?: string
}

const REPORT_URL = '/api/frontend/error'
const MAX_REPORTS_PER_MINUTE = 10
const reportTimes: number[] = []
const recentFingerprints = new Map<string, number>()

function trimField(value: unknown, maxLength: number): string {
  if (typeof value !== 'string') return ''
  return value.replaceAll(/\s+/g, ' ').trim().slice(0, maxLength)
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'string') return error
  return String(error)
}

function getErrorStack(error: unknown): string {
  if (error instanceof Error) return error.stack ?? ''
  return ''
}

function canSendReport(payload: FrontendErrorPayload): boolean {
  const now = Date.now()
  while (reportTimes.length > 0 && now - reportTimes[0] > 60_000) {
    reportTimes.shift()
  }
  if (reportTimes.length >= MAX_REPORTS_PER_MINUTE) return false

  const fingerprint = `${payload.type}:${payload.message}:${payload.source ?? ''}`
  const recentAt = recentFingerprints.get(fingerprint)
  if (recentAt && now - recentAt < 10_000) return false

  reportTimes.push(now)
  recentFingerprints.set(fingerprint, now)
  return true
}

export function reportFrontendError(payload: FrontendErrorPayload): void {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') return

  const safePayload: FrontendErrorPayload = {
    type: trimField(payload.type, 40) || 'unknown',
    message: trimField(payload.message, 500) || 'empty message',
    stack: trimField(payload.stack, 2000),
    url: trimField(window.location.href, 300),
    user_agent: trimField(navigator.userAgent, 200),
    source: trimField(payload.source, 80),
  }

  if (!canSendReport(safePayload)) return

  try {
    void fetch(REPORT_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(safePayload),
      credentials: 'omit',
      keepalive: true,
    }).catch(() => {
      /* empty */
    })
  } catch {
    /* empty */
  }
}

export function installFrontendErrorReporting(): void {
  if (typeof window === 'undefined') return

  window.addEventListener('error', (event) => {
    reportFrontendError({
      type: 'window_error',
      message: getErrorMessage(event.error ?? event.message),
      stack: getErrorStack(event.error),
      source: event.filename,
    })
  })

  window.addEventListener('unhandledrejection', (event) => {
    reportFrontendError({
      type: 'unhandled_rejection',
      message: getErrorMessage(event.reason),
      stack: getErrorStack(event.reason),
      source: 'promise',
    })
  })
}
