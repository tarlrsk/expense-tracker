// Calendar days for transactions (ADR-0071, ADR-0072). A day is text `YYYY-MM-DD`, as the API
// sends it; "today" is taken in the app time zone (ADR-0042), never the device's. Arithmetic
// is done on UTC midnights, where every day has 24 hours, so nothing shifts by a zone.

import { appTimeZone } from './format'

/** The earliest date a transaction may have (ADR-0071). */
export const minTransactionDate = '2000-01-01'

const datePattern = /^(\d{4})-(\d{2})-(\d{2})$/

/** The UTC midnight of a real calendar day written YYYY-MM-DD, or null. */
function utcOf(date: string): Date | null {
  const match = datePattern.exec(date)
  if (!match) {
    return null
  }
  const [, y = '', m = '', d = ''] = match
  const t = new Date(Date.UTC(Number(y), Number(m) - 1, Number(d)))
  // Date.UTC rolls 2026-02-30 over to 2 March; a real date comes back unchanged.
  return textOf(t) === date ? t : null
}

function utc(date: string): Date {
  const t = utcOf(date)
  if (!t) {
    throw new Error('not a date')
  }
  return t
}

function textOf(t: Date): string {
  const y = String(t.getUTCFullYear()).padStart(4, '0')
  const m = String(t.getUTCMonth() + 1).padStart(2, '0')
  const d = String(t.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** Whether `text` is a real calendar day written YYYY-MM-DD (not 2026-02-30). */
export function isDate(text: string): boolean {
  return utcOf(text) !== null
}

const todayFormat = new Intl.DateTimeFormat('en-CA', {
  timeZone: appTimeZone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

/** The day `now` falls on in Bangkok, as YYYY-MM-DD. */
export function today(now: Date = new Date()): string {
  const parts = todayFormat.formatToParts(now)
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === type)?.value ?? ''
  return `${part('year')}-${part('month')}-${part('day')}`
}

export function addDays(date: string, days: number): string {
  const t = utc(date)
  t.setUTCDate(t.getUTCDate() + days)
  return textOf(t)
}

/** The latest date a transaction may have: one year after today; 29 February gives 1 March. */
export function maxTransactionDate(todayDate: string): string {
  const t = utc(todayDate)
  return textOf(new Date(Date.UTC(t.getUTCFullYear() + 1, t.getUTCMonth(), t.getUTCDate())))
}

export type HistoryView = 'week' | 'month'

export const historyViews: readonly HistoryView[] = ['week', 'month']

/** A week (Monday to Sunday) or a calendar month; both ends included. */
export interface Period {
  view: HistoryView
  from: string
  to: string
}

/** The Monday-to-Sunday week that holds `date` (ADR-0072). */
export function weekOf(date: string): Period {
  const t = utc(date)
  // getUTCDay: Sunday 0 … Saturday 6; days since Monday: Monday 0 … Sunday 6.
  const sinceMonday = (t.getUTCDay() + 6) % 7
  const from = addDays(date, -sinceMonday)
  return { view: 'week', from, to: addDays(from, 6) }
}

/** The calendar month that holds `date`. */
export function monthOf(date: string): Period {
  const t = utc(date)
  const first = new Date(Date.UTC(t.getUTCFullYear(), t.getUTCMonth(), 1))
  const last = new Date(Date.UTC(t.getUTCFullYear(), t.getUTCMonth() + 1, 0))
  return { view: 'month', from: textOf(first), to: textOf(last) }
}

export function periodOf(view: HistoryView, date: string): Period {
  return view === 'week' ? weekOf(date) : monthOf(date)
}

/** The period `steps` weeks or months after `period` (before it when negative). */
export function shiftPeriod(period: Period, steps: number): Period {
  if (period.view === 'week') {
    return weekOf(addDays(period.from, 7 * steps))
  }
  const t = utc(period.from)
  return monthOf(textOf(new Date(Date.UTC(t.getUTCFullYear(), t.getUTCMonth() + steps, 1))))
}

export function periodContains(period: Period, date: string): boolean {
  return period.from <= date && date <= period.to
}

/** The month of a month period as the API's `month` filter, e.g. "2026-10". */
export function monthParam(period: Period): string {
  return period.from.slice(0, 7)
}

// Calendar days are formatted at their UTC midnight, so the label is the day itself. en-GB puts
// the day before the month, as Thailand does.
const dayMonthYear = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'UTC',
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})
const dayMonth = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'UTC',
  day: 'numeric',
  month: 'short',
})
const monthYear = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'UTC',
  month: 'long',
  year: 'numeric',
})
const weekdayDayMonth = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'UTC',
  weekday: 'short',
  day: 'numeric',
  month: 'short',
})
const weekdayDayMonthYear = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'UTC',
  weekday: 'short',
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})

/** "October 2026" for a month; "5 – 11 Oct 2026" or "28 Dec 2026 – 3 Jan 2027" for a week. */
export function periodLabel(period: Period): string {
  const from = utc(period.from)
  const to = utc(period.to)
  if (period.view === 'month') {
    return monthYear.format(from)
  }
  if (from.getUTCFullYear() !== to.getUTCFullYear()) {
    return `${dayMonthYear.format(from)} – ${dayMonthYear.format(to)}`
  }
  if (from.getUTCMonth() === to.getUTCMonth()) {
    return `${String(from.getUTCDate())} – ${dayMonthYear.format(to)}`
  }
  return `${dayMonth.format(from)} – ${dayMonthYear.format(to)}`
}

/** The header of a day in History: "Sat 3 Oct". */
export function dayLabel(date: string): string {
  return weekdayDayMonth.format(utc(date)).replace(',', '')
}

/** A day written in full, for one transaction: "Sat 3 Oct 2026". */
export function longDayLabel(date: string): string {
  return weekdayDayMonthYear.format(utc(date)).replace(',', '')
}
