import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, json, mockApi, noContent, transaction } from '@/test/api'

import { setTokenProvider } from './client'
import {
  createTransaction,
  deleteTransaction,
  listAllTransactions,
  listTransactions,
  updateTransaction,
} from './transactions'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

const lunch = transaction(1, { amount: '145.00', merchant: 'Noodle shop', note: 'with Kan' })

describe('listTransactions', () => {
  it.each([
    { name: 'a month', params: { month: '2026-10' }, query: 'month=2026-10' },
    {
      name: 'a week with a page size and cursor',
      params: { from: '2026-09-28', to: '2026-10-04', limit: 200, cursor: 'abc' },
      query: 'from=2026-09-28&to=2026-10-04&limit=200&cursor=abc',
    },
    { name: 'only from', params: { from: '2026-10-01' }, query: 'from=2026-10-01' },
    { name: 'nothing', params: {}, query: '' },
  ])('sends $name as the query', async ({ params, query }) => {
    api.on('GET /api/transactions', () => json(200, { transactions: [lunch], next_cursor: null }))

    await expect(listTransactions(params)).resolves.toEqual({
      transactions: [lunch],
      next_cursor: null,
    })

    expect(api.onlyCall('GET /api/transactions').query.toString()).toBe(query)
  })

  it.each([
    { name: 'no next_cursor key', body: { transactions: [] } },
    {
      name: 'a numeric amount',
      body: { transactions: [{ ...lunch, amount: 145 }], next_cursor: null },
    },
    {
      name: 'an amount without two decimals',
      body: { transactions: [{ ...lunch, amount: '145' }], next_cursor: null },
    },
    {
      name: 'a bad occurred_on',
      body: { transactions: [{ ...lunch, occurred_on: '3 Oct 2026' }], next_cursor: null },
    },
    {
      name: 'an unknown source',
      body: { transactions: [{ ...lunch, source: 'magic' }], next_cursor: null },
    },
    {
      name: 'a null merchant',
      body: { transactions: [{ ...lunch, merchant: null }], next_cursor: null },
    },
    {
      name: 'no raw_input',
      body: { transactions: [{ ...lunch, raw_input: undefined }], next_cursor: null },
    },
  ])('refuses a list with $name', async ({ body }) => {
    api.on('GET /api/transactions', () => json(200, body))

    await expect(listTransactions({ month: '2026-10' })).rejects.toMatchObject({
      code: 'bad_response',
    })
  })
})

describe('listAllTransactions', () => {
  it('follows next_cursor until the last page', async () => {
    const pages: Record<string, { transactions: unknown[]; next_cursor: string | null }> = {
      '': { transactions: [transaction(1), transaction(2)], next_cursor: 'c1' },
      c1: { transactions: [transaction(3)], next_cursor: 'c2' },
      c2: { transactions: [transaction(4)], next_cursor: null },
    }
    api.on('GET /api/transactions', (call) =>
      json(200, pages[call.query.get('cursor') ?? ''] ?? {}),
    )

    const all = await listAllTransactions({ from: '2026-09-28', to: '2026-10-04' })

    expect(all.map((t) => t.id)).toEqual([1, 2, 3, 4].map((n) => transaction(n).id))
    const queries = api.callsTo('GET /api/transactions').map((c) => c.query.toString())
    expect(queries).toEqual([
      'from=2026-09-28&to=2026-10-04&limit=200',
      'from=2026-09-28&to=2026-10-04&limit=200&cursor=c1',
      'from=2026-09-28&to=2026-10-04&limit=200&cursor=c2',
    ])
  })

  it('stops with an error when a cursor comes back again', async () => {
    api.on('GET /api/transactions', () =>
      json(200, { transactions: [transaction(1)], next_cursor: 'same' }),
    )

    await expect(listAllTransactions({ month: '2026-10' })).rejects.toMatchObject({
      code: 'bad_response',
    })
    expect(api.callsTo('GET /api/transactions')).toHaveLength(2)
  })

  it('passes a failed page through', async () => {
    api.on('GET /api/transactions', (call) =>
      call.query.has('cursor')
        ? apiError(500, 'internal', 'internal error')
        : json(200, { transactions: [transaction(1)], next_cursor: 'c1' }),
    )

    await expect(listAllTransactions({ month: '2026-10' })).rejects.toMatchObject({
      code: 'internal',
    })
  })
})

describe('createTransaction', () => {
  const req = {
    id: lunch.id,
    amount: '145.00',
    occurred_on: '2026-10-03',
    category_id: lunch.category_id,
    merchant: 'Noodle shop',
  }

  it.each([
    { status: 201, name: 'a new one' },
    { status: 200, name: 'one already saved with this id' },
  ])('posts the transaction and returns $name', async ({ status }) => {
    api.on('POST /api/transactions', () => json(status, lunch))

    await expect(createTransaction(req)).resolves.toEqual(lunch)

    expect(api.onlyCall('POST /api/transactions').body).toEqual(req)
  })

  it('passes a rule error through', async () => {
    api.on('POST /api/transactions', () =>
      apiError(
        400,
        'invalid_input',
        'choose one of your active categories (create or unarchive one if there is none)',
      ),
    )

    await expect(createTransaction(req)).rejects.toMatchObject({ code: 'invalid_input' })
  })

  it('sends a quick-entry transaction with its source and typed text', async () => {
    const typed = { ...lunch, source: 'text', raw_input: 'noodles 145' }
    api.on('POST /api/transactions', () => json(201, typed))
    const textReq = { ...req, source: 'text' as const, raw_input: 'noodles 145' }

    await expect(createTransaction(textReq)).resolves.toEqual(typed)

    expect(api.onlyCall('POST /api/transactions').body).toEqual(textReq)
  })
})

describe('updateTransaction', () => {
  it('patches only the fields given', async () => {
    api.on(`PATCH /api/transactions/${lunch.id}`, () => json(200, { ...lunch, note: '' }))

    await expect(updateTransaction(lunch.id, { note: '' })).resolves.toEqual({
      ...lunch,
      note: '',
    })

    expect(api.onlyCall(`PATCH /api/transactions/${lunch.id}`).body).toEqual({ note: '' })
  })
})

describe('deleteTransaction', () => {
  it('deletes the escaped path', async () => {
    api.on(`DELETE /api/transactions/${lunch.id}`, () => noContent())

    await expect(deleteTransaction(lunch.id)).resolves.toBeUndefined()

    await deleteTransaction('a/b').catch(() => undefined)
    expect(api.calls.at(-1)?.path).toBe('/api/transactions/a%2Fb')
  })

  it('passes not_found through', async () => {
    api.on(`DELETE /api/transactions/${lunch.id}`, () =>
      apiError(404, 'not_found', 'there is no such transaction'),
    )

    await expect(deleteTransaction(lunch.id)).rejects.toMatchObject({
      code: 'not_found',
      status: 404,
    })
  })
})
