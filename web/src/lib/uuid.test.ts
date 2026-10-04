import { describe, expect, it } from 'vitest'

import { uuidv7 } from './uuid'

const v7Pattern = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

const zeros = (bytes: Uint8Array) => {
  bytes.fill(0)
}
const ones = (bytes: Uint8Array) => {
  bytes.fill(0xff)
}

describe('uuidv7', () => {
  it.each([
    {
      name: 'the epoch, zero randomness',
      now: 0,
      fill: zeros,
      want: '00000000-0000-7000-8000-000000000000',
    },
    {
      name: 'the epoch, full randomness',
      now: 0,
      fill: ones,
      want: '00000000-0000-7fff-bfff-ffffffffffff',
    },
    // 2026-10-04T00:00:00.000Z is 1791072000000 ms, 0x01a10435d800.
    {
      name: 'a 2026 time',
      now: Date.UTC(2026, 9, 4),
      fill: zeros,
      want: '01a10435-d800-7000-8000-000000000000',
    },
    {
      name: 'the largest 48-bit time',
      now: 2 ** 48 - 1,
      fill: zeros,
      want: 'ffffffff-ffff-7000-8000-000000000000',
    },
  ])('writes $name', ({ now, fill, want }) => {
    expect(uuidv7(now, fill)).toBe(want)
  })

  it('has the usual 36-character form, version 7 and the RFC variant', () => {
    for (let i = 0; i < 100; i++) {
      const id = uuidv7()
      expect(id).toHaveLength(36)
      expect(id).toMatch(v7Pattern)
    }
  })

  it('makes a different id each time', () => {
    const ids = new Set(Array.from({ length: 500 }, () => uuidv7(1_000)))
    expect(ids.size).toBe(500)
  })

  it('sorts a later id after an earlier one', () => {
    const earlier = uuidv7(Date.UTC(2026, 0, 1), ones)
    const later = uuidv7(Date.UTC(2026, 0, 1) + 1, zeros)
    expect(earlier < later).toBe(true)
  })
})
