// Dates are shown in the app time zone (ADR-0042, ADR-0072), never the device's: a phone set to
// another zone still sees Bangkok dates.

export const appTimeZone = 'Asia/Bangkok'

// en-GB writes the day before the month, as Thailand does: "3 Oct 2026".
const dateFormat = new Intl.DateTimeFormat('en-GB', {
  timeZone: appTimeZone,
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})

const dateTimeFormat = new Intl.DateTimeFormat('en-GB', {
  timeZone: appTimeZone,
  day: 'numeric',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
})

/** An RFC 3339 timestamp as a Bangkok date, e.g. "3 Oct 2026". */
export function formatDate(timestamp: string): string {
  return dateFormat.format(new Date(timestamp))
}

/** An RFC 3339 timestamp as a Bangkok date and time, e.g. "3 Oct 2026, 14:05". */
export function formatDateTime(timestamp: string): string {
  return dateTimeFormat.format(new Date(timestamp))
}
