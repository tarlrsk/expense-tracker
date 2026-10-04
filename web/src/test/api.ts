// A fake /api for tests: each handler answers one "METHOD /path" and every call is recorded.

import { vi } from 'vitest'

export interface RecordedCall {
  method: string
  path: string
  /** The query string, e.g. `month=2026-10`. */
  query: URLSearchParams
  headers: Headers
  body: unknown
}

export type Handler = (call: RecordedCall) => Response | Promise<Response>

export function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

export function noContent(): Response {
  return new Response(null, { status: 204 })
}

export function apiError(status: number, code: string, message: string): Response {
  return json(status, { error: { code, message } })
}

/**
 * Replaces fetch with a fake API. An unknown route fails the test loudly with a 404 body,
 * and a handler may be swapped per test with `on`.
 */
export function mockApi(initial: Record<string, Handler> = {}) {
  const handlers = new Map(Object.entries(initial))
  const calls: RecordedCall[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const parsed = new URL(url, 'http://localhost')
    const path = parsed.pathname
    const method = init.method ?? 'GET'
    const body: unknown = typeof init.body === 'string' ? JSON.parse(init.body) : undefined
    const call = {
      method,
      path,
      query: parsed.searchParams,
      headers: new Headers(init.headers),
      body,
    }
    calls.push(call)
    const handler = handlers.get(`${method} ${path}`)
    if (!handler) {
      return apiError(404, 'not_found', `no test handler for ${method} ${path}`)
    }
    return handler(call)
  })
  vi.stubGlobal('fetch', fetchMock)
  return {
    calls,
    fetchMock,
    on(route: string, handler: Handler) {
      handlers.set(route, handler)
    },
    callsTo(route: string): RecordedCall[] {
      return calls.filter((c) => `${c.method} ${c.path}` === route)
    },
    /** The one call to `route`; fails when there were none or several. */
    onlyCall(route: string): RecordedCall {
      const matching = calls.filter((c) => `${c.method} ${c.path}` === route)
      const first = matching.at(0)
      if (matching.length !== 1 || first === undefined) {
        throw new Error(`${String(matching.length)} calls to ${route}, want 1`)
      }
      return first
    },
  }
}

export const profile = {
  id: '0190a1b2-0000-7000-8000-000000000001',
  email: 'kanya@example.com',
  display_name: 'Kanya',
  role: 'user',
  created_at: '2026-10-01T03:00:00Z',
}

export const operatorProfile = { ...profile, role: 'operator' }

export const session = { token: 'tok-new', expires_at: '2026-11-02T03:00:00Z' }

/** A category as the API sends it; `n` makes the id unique. */
export function category(
  n: number,
  fields: Partial<{
    name: string
    icon: string
    kind: 'expense' | 'income'
    archived: boolean
    sort_order: number
  }> = {},
) {
  return {
    id: `0190a1b2-0000-7000-8000-1000000000${String(n).padStart(2, '0')}`,
    name: `Category ${String(n)}`,
    icon: '📦',
    kind: 'expense' as 'expense' | 'income',
    archived: false,
    sort_order: n,
    created_at: '2026-10-01T03:00:00Z',
    updated_at: '2026-10-01T03:00:00Z',
    ...fields,
  }
}

/** A transaction as the API sends it; `n` makes the id unique. */
export function transaction(
  n: number,
  fields: Partial<{
    amount: string
    occurred_on: string
    merchant: string
    category_id: string
    note: string
    source: 'manual' | 'text' | 'scan' | 'csv'
    raw_input: string
  }> = {},
) {
  return {
    id: `0190a1b2-0000-7000-8000-2000000000${String(n).padStart(2, '0')}`,
    owner_id: profile.id,
    amount: '100.00',
    currency: 'THB',
    occurred_on: '2026-10-03',
    merchant: '',
    category_id: category(1).id,
    note: '',
    source: 'manual',
    raw_input: '',
    created_at: '2026-10-03T03:00:00Z',
    updated_at: '2026-10-03T03:00:00Z',
    ...fields,
  }
}
