import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, mockApi } from '@/test/api'

import { ApiError, setTokenProvider } from './client'
import { defaultExportFileName, getTransactionsCsv } from './export'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

const header = '﻿Date,Amount,Kind,Category,Description,Note,Source\r\n'

function csv(body: string, fileName: string | null = 'satang-transactions.csv'): Response {
  const headers = new Headers({ 'Content-Type': 'text/csv; charset=utf-8' })
  if (fileName !== null) {
    headers.set('Content-Disposition', `attachment; filename="${fileName}"`)
  }
  return new Response(body, { status: 200, headers })
}

describe('getTransactionsCsv', () => {
  it('asks for every date when no period is given', async () => {
    api.on('GET /api/export.csv', () =>
      csv(`${header}2026-10-01,145.00,expense,Food,Grab,,manual\r\n`),
    )

    const file = await getTransactionsCsv()

    const call = api.onlyCall('GET /api/export.csv')
    expect([...call.query.keys()]).toEqual([])
    expect(call.headers.get('Authorization')).toBe('Bearer tok')
    expect(file.fileName).toBe('satang-transactions.csv')
    expect(file.empty).toBe(false)
    expect(await file.blob.text()).toContain('2026-10-01,145.00')
  })

  it('sends from and to and keeps the name the API gives', async () => {
    const name = 'satang-transactions-2026-10-01-to-2026-10-31.csv'
    api.on('GET /api/export.csv', () =>
      csv(`${header}2026-10-02,60.00,expense,Food,Coffee,,text\r\n`, name),
    )

    const file = await getTransactionsCsv({ from: '2026-10-01', to: '2026-10-31' })

    const call = api.onlyCall('GET /api/export.csv')
    expect(call.query.get('from')).toBe('2026-10-01')
    expect(call.query.get('to')).toBe('2026-10-31')
    expect(file.fileName).toBe(name)
  })

  it('falls back to the default name when the response suggests none', async () => {
    api.on('GET /api/export.csv', () => csv(header, null))

    const file = await getTransactionsCsv()

    expect(file.fileName).toBe(defaultExportFileName)
  })

  it.each([
    { name: 'only the header', body: header, empty: true },
    { name: 'the header without a line end', body: header.trimEnd(), empty: true },
    { name: 'nothing at all', body: '', empty: true },
    { name: 'one row', body: `${header}2026-10-01,1.00,expense,Food,a,,manual\r\n`, empty: false },
  ])('says whether the file is empty: $name', async ({ body, empty }) => {
    api.on('GET /api/export.csv', () => csv(body))

    expect((await getTransactionsCsv()).empty).toBe(empty)
  })

  it('throws the API error on a bad period', async () => {
    api.on('GET /api/export.csv', () => apiError(400, 'invalid_input', 'from must not be after to'))

    await expect(getTransactionsCsv({ from: '2026-10-31', to: '2026-10-01' })).rejects.toEqual(
      expect.objectContaining({ code: 'invalid_input' }),
    )
    await expect(getTransactionsCsv()).rejects.toBeInstanceOf(ApiError)
  })
})
