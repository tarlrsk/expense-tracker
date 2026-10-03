import { describe, expect, it } from 'vitest'

import { amountParts } from './amount'

describe('amountParts', () => {
  it.each([
    { text: '0.50', want: { sign: '', baht: '0', satang: '50' } },
    { text: '145.00', want: { sign: '', baht: '145', satang: '00' } },
    { text: '1000.00', want: { sign: '', baht: '1,000', satang: '00' } },
    { text: '1234567.05', want: { sign: '', baht: '1,234,567', satang: '05' } },
    { text: '9999999999.99', want: { sign: '', baht: '9,999,999,999', satang: '99' } },
    { text: '00012.30', want: { sign: '', baht: '12', satang: '30' } },
    { text: '-12.30', want: { sign: '-', baht: '12', satang: '30' } },
  ])('splits $text', ({ text, want }) => {
    expect(amountParts(text)).toEqual(want)
  })

  it.each(['', '145', '145.5', '145.505', '1e3', 'abc', '1,000.00', '.50'])(
    'refuses %j',
    (text) => {
      expect(amountParts(text)).toBeNull()
    },
  )

  it('keeps digits a float would lose', () => {
    // 90071992547409.93 is not exactly representable as a double.
    expect(amountParts('90071992547409.93')).toEqual({
      sign: '',
      baht: '90,071,992,547,409',
      satang: '93',
    })
  })
})
