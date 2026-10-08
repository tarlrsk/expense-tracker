import { describe, expect, it } from 'vitest'

import { barPercent, shareLabel } from './share'

describe('shareLabel', () => {
  it.each([
    { part: '100.00', whole: '100.00', want: '100%' },
    { part: '50.00', whole: '100.00', want: '50%' },
    { part: '1.00', whole: '3.00', want: '33%' },
    { part: '2.00', whole: '3.00', want: '67%' },
    // Exactly half a percent rounds up; just below it rounds down to "<1%".
    { part: '0.50', whole: '100.00', want: '1%' },
    { part: '0.49', whole: '100.00', want: '<1%' },
    { part: '0.01', whole: '9999999999.99', want: '<1%' },
    { part: '12.50', whole: '1000.00', want: '1%' },
    { part: '24.99', whole: '1000.00', want: '2%' },
    { part: '25.00', whole: '1000.00', want: '3%' },
    { part: '99.49', whole: '100.00', want: '99%' },
    { part: '99.50', whole: '100.00', want: '100%' },
    // 0.1 + 0.2 of 0.3 in floats is not exactly 1; in satang it is.
    { part: '0.30', whole: '0.30', want: '100%' },
    { part: '0.00', whole: '100.00', want: '0%' },
    { part: '5.00', whole: '0.00', want: null },
    { part: '5.00', whole: '-1.00', want: null },
  ])('$part of $whole is $want', ({ part, whole, want }) => {
    expect(shareLabel(part, whole)).toBe(want)
  })
})

describe('barPercent', () => {
  it.each([
    { part: '500.00', max: '500.00', want: 100 },
    { part: '250.00', max: '500.00', want: 50 },
    { part: '1.00', max: '3.00', want: 33.33 },
    { part: '0.01', max: '999999.99', want: 0 },
    { part: '10.00', max: '1000.00', want: 1 },
    { part: '0.00', max: '500.00', want: 0 },
    { part: '5.00', max: '0.00', want: 0 },
    { part: '600.00', max: '500.00', want: 100 },
  ])('$part next to $max is $want%', ({ part, max, want }) => {
    expect(barPercent(part, max)).toBe(want)
  })
})
