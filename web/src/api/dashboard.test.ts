import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, json, mockApi } from '@/test/api'

import { setTokenProvider } from './client'
import { getDashboard } from './dashboard'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

const food = {
  category_id: '0190a1b2-0000-7000-8000-100000000001',
  name: 'Food',
  icon: '🍜',
  archived: false,
  total: '1450.00',
  count: 12,
}
const old = { ...food, category_id: '0190a1b2-0000-7000-8000-100000000002', archived: true }
const october = {
  month: '2026-10',
  totals: { spent: '1450.00', income: '1000.00', difference: '-450.00' },
  categories: [food, old],
  trend: [
    { month: '2026-05', spent: '0.00' },
    { month: '2026-06', spent: '10.50' },
    { month: '2026-07', spent: '0.00' },
    { month: '2026-08', spent: '0.00' },
    { month: '2026-09', spent: '99.00' },
    { month: '2026-10', spent: '1450.00' },
  ],
  top_descriptions: [{ description: 'Noodle shop', total: '600.00', count: 4 }],
}

describe('getDashboard', () => {
  it('gets the month with the bearer token and keeps amounts as text', async () => {
    api.on('GET /api/dashboard', () => json(200, october))

    await expect(getDashboard('2026-10')).resolves.toEqual(october)

    const call = api.onlyCall('GET /api/dashboard')
    expect(call.query.toString()).toBe('month=2026-10')
    expect(call.headers.get('Authorization')).toBe('Bearer tok')
  })

  it('takes an empty month', async () => {
    const empty = {
      ...october,
      totals: { spent: '0.00', income: '0.00', difference: '0.00' },
      categories: [],
      top_descriptions: [],
    }
    api.on('GET /api/dashboard', () => json(200, empty))

    await expect(getDashboard('2026-10')).resolves.toEqual(empty)
  })

  it.each([
    { name: 'no month', body: { ...october, month: undefined } },
    { name: 'a month with a day', body: { ...october, month: '2026-10-01' } },
    { name: 'month 13', body: { ...october, month: '2026-13' } },
    { name: 'no totals', body: { ...october, totals: undefined } },
    { name: 'totals as a list', body: { ...october, totals: [] } },
    {
      name: 'a numeric spent',
      body: { ...october, totals: { ...october.totals, spent: 1450 } },
    },
    {
      name: 'an income without two decimals',
      body: { ...october, totals: { ...october.totals, income: '1000' } },
    },
    {
      name: 'no difference',
      body: { ...october, totals: { spent: '1.00', income: '0.00' } },
    },
    { name: 'no categories', body: { ...october, categories: undefined } },
    { name: 'a null category', body: { ...october, categories: [null] } },
    {
      name: 'an empty category_id',
      body: { ...october, categories: [{ ...food, category_id: '' }] },
    },
    { name: 'a null name', body: { ...october, categories: [{ ...food, name: null }] } },
    { name: 'no icon', body: { ...october, categories: [{ ...food, icon: undefined }] } },
    { name: 'a text archived', body: { ...october, categories: [{ ...food, archived: 'no' }] } },
    { name: 'a numeric total', body: { ...october, categories: [{ ...food, total: 1450 }] } },
    { name: 'a fractional count', body: { ...october, categories: [{ ...food, count: 1.5 }] } },
    { name: 'no trend', body: { ...october, trend: undefined } },
    { name: 'a bad trend month', body: { ...october, trend: [{ month: 'May', spent: '0.00' }] } },
    { name: 'a trend without spent', body: { ...october, trend: [{ month: '2026-10' }] } },
    { name: 'no top_descriptions', body: { ...october, top_descriptions: undefined } },
    {
      name: 'a null description',
      body: { ...october, top_descriptions: [{ description: null, total: '1.00', count: 1 }] },
    },
    {
      name: 'a description with a text count',
      body: { ...october, top_descriptions: [{ description: 'a', total: '1.00', count: '1' }] },
    },
  ])('refuses a body with $name', async ({ body }) => {
    api.on('GET /api/dashboard', () => json(200, body))

    await expect(getDashboard('2026-10')).rejects.toMatchObject({ code: 'bad_response' })
  })

  it('passes a refused month through as invalid_input', async () => {
    api.on('GET /api/dashboard', () =>
      apiError(
        400,
        'invalid_input',
        'month must be written YYYY-MM, from 2000-01 to one year after this month',
      ),
    )

    await expect(getDashboard('1999-12')).rejects.toMatchObject({
      code: 'invalid_input',
      status: 400,
    })
  })
})
