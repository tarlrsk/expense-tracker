import { describe, expect, it } from 'vitest'

import { parseAmountInput, satangToText, sumAmounts, toSatang } from './money'

describe('parseAmountInput', () => {
  it.each([
    { text: '145', value: '145.00' },
    { text: '145.5', value: '145.50' },
    { text: '145.50', value: '145.50' },
    { text: '0.01', value: '0.01' },
    { text: '.5', value: '0.50' },
    { text: '5.', value: '5.00' },
    { text: '  42  ', value: '42.00' },
    { text: '007.10', value: '7.10' },
    { text: '9999999999.99', value: '9999999999.99' },
    { text: '0009999999999.99', value: '9999999999.99' },
  ])('takes $text as $value', ({ text, value }) => {
    expect(parseAmountInput(text)).toEqual({ ok: true, value })
  })

  it.each([
    { text: '', error: 'empty' },
    { text: '   ', error: 'empty' },
    { text: '.', error: 'format' },
    { text: '1,000', error: 'format' },
    { text: '1.000,50', error: 'format' },
    { text: '-5', error: 'format' },
    { text: '+5', error: 'format' },
    { text: '1e3', error: 'format' },
    { text: '12 34', error: 'format' },
    { text: 'abc', error: 'format' },
    { text: '1.2.3', error: 'format' },
    { text: '145.505', error: 'decimals' },
    { text: '0.001', error: 'decimals' },
    { text: '0', error: 'zero' },
    { text: '0.00', error: 'zero' },
    { text: '.0', error: 'zero' },
    { text: '10000000000', error: 'too_large' },
    { text: '10000000000.00', error: 'too_large' },
    { text: '99999999999999999999', error: 'too_large' },
  ])('refuses $text: $error', ({ text, error }) => {
    expect(parseAmountInput(text)).toEqual({ ok: false, error })
  })
})

describe('toSatang and satangToText', () => {
  it.each([
    { text: '0.00', satang: 0n },
    { text: '0.05', satang: 5n },
    { text: '145.00', satang: 14500n },
    { text: '9999999999.99', satang: 999999999999n },
    { text: '-12.30', satang: -1230n },
    { text: '90071992547409.93', satang: 9007199254740993n },
  ])('$text is $satang satang', ({ text, satang }) => {
    expect(toSatang(text)).toBe(satang)
    expect(satangToText(satang)).toBe(text)
  })

  it.each(['145', '145.5', '1,000.00', '', 'x'])('toSatang refuses %j', (text) => {
    expect(() => toSatang(text)).toThrow()
  })
})

describe('sumAmounts', () => {
  it.each([
    { amounts: [], sum: '0.00' },
    { amounts: ['145.00'], sum: '145.00' },
    // As floats, 0.1 + 0.2 is 0.30000000000000004.
    { amounts: ['0.10', '0.20'], sum: '0.30' },
    { amounts: ['0.99', '0.01', '1.50'], sum: '2.50' },
    { amounts: ['9999999999.99', '9999999999.99'], sum: '19999999999.98' },
    { amounts: Array.from({ length: 1000 }, () => '0.01'), sum: '10.00' },
  ])('adds $amounts.length amounts to $sum', ({ amounts, sum }) => {
    expect(sumAmounts(amounts)).toBe(sum)
  })
})
