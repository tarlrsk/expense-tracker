import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, category, json, mockApi } from '@/test/api'

import { setTokenProvider } from './client'
import { parseEntry } from './entry'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

const coffee = {
  text: 'coffee 60',
  amount: '60.00',
  occurred_on: '2026-10-04',
  merchant: 'coffee',
  category_id: category(1).id,
  confidence: 'high',
  resolved_by: 'rule',
}

// An item the parser could not finish: unread fields are "" and the category null.
const unread = {
  text: 'something',
  amount: '',
  occurred_on: '',
  merchant: 'something',
  category_id: null,
  confidence: 'low',
  resolved_by: 'none',
}

describe('parseEntry', () => {
  it('posts the text with the bearer token and returns the proposals', async () => {
    api.on('POST /api/entry/parse', () => json(200, { items: [coffee, unread], ai: 'used' }))

    await expect(parseEntry('coffee 60, something')).resolves.toEqual({
      items: [coffee, unread],
      ai: 'used',
    })

    const call = api.onlyCall('POST /api/entry/parse')
    expect(call.body).toEqual({ text: 'coffee 60, something' })
    expect(call.headers.get('Authorization')).toBe('Bearer tok')
  })

  it.each(['not_needed', 'used', 'limit_reached', 'unavailable', 'not_configured'])(
    'accepts ai %s',
    async (ai) => {
      api.on('POST /api/entry/parse', () => json(200, { items: [coffee], ai }))

      await expect(parseEntry('coffee 60')).resolves.toMatchObject({ ai })
    },
  )

  it.each([
    { name: 'no items key', body: { ai: 'used' } },
    { name: 'no ai key', body: { items: [coffee] } },
    { name: 'an unknown ai', body: { items: [coffee], ai: 'maybe' } },
    { name: 'a numeric amount', body: { items: [{ ...coffee, amount: 60 }], ai: 'used' } },
    {
      name: 'an amount without decimals',
      body: { items: [{ ...coffee, amount: '60' }], ai: 'used' },
    },
    { name: 'a negative amount', body: { items: [{ ...coffee, amount: '-60.00' }], ai: 'used' } },
    {
      name: 'a bad occurred_on',
      body: { items: [{ ...coffee, occurred_on: '4 Oct' }], ai: 'used' },
    },
    { name: 'an empty category_id', body: { items: [{ ...coffee, category_id: '' }], ai: 'used' } },
    {
      name: 'a missing category_id',
      body: { items: [{ ...coffee, category_id: undefined }], ai: 'used' },
    },
    {
      name: 'an unknown confidence',
      body: { items: [{ ...coffee, confidence: 'mid' }], ai: 'used' },
    },
    {
      name: 'an unknown resolved_by',
      body: { items: [{ ...coffee, resolved_by: 'x' }], ai: 'used' },
    },
    { name: 'a null merchant', body: { items: [{ ...coffee, merchant: null }], ai: 'used' } },
  ])('refuses an answer with $name', async ({ body }) => {
    api.on('POST /api/entry/parse', () => json(200, body))

    await expect(parseEntry('coffee 60')).rejects.toMatchObject({ code: 'bad_response' })
  })

  it('passes a rule error through', async () => {
    api.on('POST /api/entry/parse', () =>
      apiError(400, 'invalid_input', 'text must hold at most 20 items'),
    )

    await expect(parseEntry('a 1')).rejects.toMatchObject({
      code: 'invalid_input',
      message: 'text must hold at most 20 items',
    })
  })
})
