import { describe, expect, it } from 'vitest'

import {
  addDays,
  dayLabel,
  isDate,
  longDayLabel,
  maxTransactionDate,
  monthOf,
  monthParam,
  periodContains,
  periodLabel,
  periodOf,
  shiftPeriod,
  today,
  weekOf,
} from './dates'

describe('today in Asia/Bangkok', () => {
  it.each([
    { now: '2026-10-02T16:59:59Z', want: '2026-10-02' },
    // 17:00 UTC is midnight in Bangkok (UTC+7).
    { now: '2026-10-02T17:00:00Z', want: '2026-10-03' },
    { now: '2026-12-31T17:00:00Z', want: '2027-01-01' },
    { now: '2028-02-28T18:00:00Z', want: '2028-02-29' },
  ])('$now is $want', ({ now, want }) => {
    expect(today(new Date(now))).toBe(want)
  })
})

describe('isDate', () => {
  it.each([
    { text: '2026-10-03', want: true },
    { text: '2028-02-29', want: true },
    { text: '2000-01-01', want: true },
    { text: '2026-02-29', want: false },
    { text: '2026-02-30', want: false },
    { text: '2026-13-01', want: false },
    { text: '2026-1-3', want: false },
    { text: '03/10/2026', want: false },
    { text: '', want: false },
  ])('$text: $want', ({ text, want }) => {
    expect(isDate(text)).toBe(want)
  })
})

describe('addDays', () => {
  it.each([
    { date: '2026-10-03', days: 1, want: '2026-10-04' },
    { date: '2026-10-31', days: 1, want: '2026-11-01' },
    { date: '2026-12-31', days: 1, want: '2027-01-01' },
    { date: '2026-03-01', days: -1, want: '2026-02-28' },
    { date: '2028-03-01', days: -1, want: '2028-02-29' },
    { date: '2026-10-03', days: -7, want: '2026-09-26' },
  ])('$date + $days = $want', ({ date, days, want }) => {
    expect(addDays(date, days)).toBe(want)
  })
})

describe('maxTransactionDate', () => {
  it.each([
    { today: '2026-10-04', want: '2027-10-04' },
    { today: '2026-12-31', want: '2027-12-31' },
    // As the API does (Go's AddDate): 29 February plus a year is 1 March.
    { today: '2028-02-29', want: '2029-03-01' },
  ])('$today → $want', ({ today: t, want }) => {
    expect(maxTransactionDate(t)).toBe(want)
  })
})

describe('weekOf: Monday to Sunday', () => {
  it.each([
    { date: '2026-10-05', from: '2026-10-05', to: '2026-10-11', name: 'a Monday' },
    { date: '2026-10-04', from: '2026-09-28', to: '2026-10-04', name: 'a Sunday' },
    { date: '2026-10-03', from: '2026-09-28', to: '2026-10-04', name: 'a Saturday' },
    { date: '2027-01-01', from: '2026-12-28', to: '2027-01-03', name: 'across a year' },
    { date: '2028-03-01', from: '2028-02-28', to: '2028-03-05', name: 'across 29 February' },
  ])('$name ($date)', ({ date, from, to }) => {
    expect(weekOf(date)).toEqual({ view: 'week', from, to })
  })
})

describe('monthOf', () => {
  it.each([
    { date: '2026-10-04', from: '2026-10-01', to: '2026-10-31' },
    { date: '2026-02-14', from: '2026-02-01', to: '2026-02-28' },
    { date: '2028-02-01', from: '2028-02-01', to: '2028-02-29' },
    { date: '2026-12-31', from: '2026-12-01', to: '2026-12-31' },
  ])('$date', ({ date, from, to }) => {
    expect(monthOf(date)).toEqual({ view: 'month', from, to })
  })

  it('gives the month filter', () => {
    expect(monthParam(monthOf('2026-10-04'))).toBe('2026-10')
  })
})

describe('shiftPeriod', () => {
  it.each([
    { period: weekOf('2026-10-04'), steps: -1, from: '2026-09-21', to: '2026-09-27' },
    { period: weekOf('2026-10-04'), steps: 1, from: '2026-10-05', to: '2026-10-11' },
    { period: weekOf('2026-12-30'), steps: 1, from: '2027-01-04', to: '2027-01-10' },
    { period: monthOf('2026-10-04'), steps: -1, from: '2026-09-01', to: '2026-09-30' },
    { period: monthOf('2026-12-04'), steps: 1, from: '2027-01-01', to: '2027-01-31' },
    { period: monthOf('2026-03-31'), steps: -1, from: '2026-02-01', to: '2026-02-28' },
    { period: monthOf('2026-01-15'), steps: -13, from: '2024-12-01', to: '2024-12-31' },
  ])('$period.view from $period.from by $steps', ({ period, steps, from, to }) => {
    expect(shiftPeriod(period, steps)).toEqual({ view: period.view, from, to })
  })
})

describe('periodOf and periodContains', () => {
  it('picks the week or month of a day, and knows its ends', () => {
    const week = periodOf('week', '2026-10-01')
    expect(week).toEqual(weekOf('2026-10-01'))
    expect(periodContains(week, '2026-09-28')).toBe(true)
    expect(periodContains(week, '2026-10-04')).toBe(true)
    expect(periodContains(week, '2026-10-05')).toBe(false)
    expect(periodOf('month', '2026-10-01')).toEqual(monthOf('2026-10-01'))
  })
})

describe('labels', () => {
  it.each([
    { period: monthOf('2026-10-04'), want: 'October 2026' },
    { period: weekOf('2026-10-07'), want: '5 – 11 Oct 2026' },
    // en-GB abbreviates September as "Sept".
    { period: weekOf('2026-10-04'), want: '28 Sept – 4 Oct 2026' },
    { period: weekOf('2027-01-01'), want: '28 Dec 2026 – 3 Jan 2027' },
  ])('period $period.view $period.from: $want', ({ period, want }) => {
    expect(periodLabel(period)).toBe(want)
  })

  it.each([
    { date: '2026-10-03', day: 'Sat 3 Oct', long: 'Sat 3 Oct 2026' },
    { date: '2026-10-05', day: 'Mon 5 Oct', long: 'Mon 5 Oct 2026' },
    { date: '2027-01-01', day: 'Fri 1 Jan', long: 'Fri 1 Jan 2027' },
  ])('day $date', ({ date, day, long }) => {
    expect(dayLabel(date)).toBe(day)
    expect(longDayLabel(date)).toBe(long)
  })
})
