import { describe, expect, it } from 'vitest'

import { formatDate, formatDateTime } from './format'

describe('dates in Asia/Bangkok', () => {
  it.each([
    // 20:30 UTC is already the next day in Bangkok (UTC+7).
    { ts: '2026-10-02T20:30:00Z', date: '3 Oct 2026', dateTime: '3 Oct 2026, 03:30' },
    { ts: '2026-10-03T07:05:00Z', date: '3 Oct 2026', dateTime: '3 Oct 2026, 14:05' },
    { ts: '2026-12-31T16:59:00Z', date: '31 Dec 2026', dateTime: '31 Dec 2026, 23:59' },
    { ts: '2026-12-31T17:00:00Z', date: '1 Jan 2027', dateTime: '1 Jan 2027, 00:00' },
  ])('$ts', ({ ts, date, dateTime }) => {
    expect(formatDate(ts)).toBe(date)
    expect(formatDateTime(ts)).toBe(dateTime)
  })
})
