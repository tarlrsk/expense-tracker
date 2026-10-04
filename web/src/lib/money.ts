// Money typed by the user and summed for display. Amounts are text such as "145.00" (ADR-0071)
// and are summed in whole satang with BigInt: never as floats.

/** The largest amount the API takes: 9,999,999,999.99 (ADR-0071), in satang. */
export const maxAmountSatang = 999_999_999_999n

/** Why an amount typed by the user was refused. */
export type AmountInputError = 'empty' | 'format' | 'decimals' | 'zero' | 'too_large'

export type AmountInputResult = { ok: true; value: string } | { ok: false; error: AmountInputError }

// Digits with an optional point and fraction; ".5" and "5." are accepted as 0.50 and 5.00.
const inputPattern = /^(\d*)(?:\.(\d*))?$/

/**
 * Reads the amount field: more than 0, at most two decimals, at most 9,999,999,999.99 (the API's
 * rule, ADR-0071). Spaces around it are ignored; nothing is ever rounded. A good amount comes
 * back normalised to two decimals for sending, e.g. "145" → "145.00".
 */
export function parseAmountInput(text: string): AmountInputResult {
  const trimmed = text.trim()
  if (trimmed === '') {
    return { ok: false, error: 'empty' }
  }
  const match = inputPattern.exec(trimmed)
  if (!match) {
    return { ok: false, error: 'format' }
  }
  const [, whole = '', fraction = ''] = match
  if (whole === '' && fraction === '') {
    return { ok: false, error: 'format' }
  }
  if (fraction.length > 2) {
    return { ok: false, error: 'decimals' }
  }
  const satang = BigInt(whole === '' ? '0' : whole) * 100n + BigInt(fraction.padEnd(2, '0'))
  if (satang === 0n) {
    return { ok: false, error: 'zero' }
  }
  if (satang > maxAmountSatang) {
    return { ok: false, error: 'too_large' }
  }
  return { ok: true, value: satangToText(satang) }
}

const apiAmountPattern = /^(-?)(\d+)\.(\d{2})$/

/** "145.05" → 14505n. Throws on text that is not an amount in the API's form. */
export function toSatang(amount: string): bigint {
  const match = apiAmountPattern.exec(amount)
  if (!match) {
    throw new Error('not an amount')
  }
  const [, sign = '', whole = '0', fraction = '00'] = match
  const value = BigInt(whole) * 100n + BigInt(fraction)
  return sign === '-' ? -value : value
}

/** 14505n → "145.05", in the API's form. */
export function satangToText(satang: bigint): string {
  const negative = satang < 0n
  const abs = negative ? -satang : satang
  const whole = (abs / 100n).toString()
  const fraction = (abs % 100n).toString().padStart(2, '0')
  return `${negative ? '-' : ''}${whole}.${fraction}`
}

/** The exact sum of amounts written as "145.00", as text in the same form. */
export function sumAmounts(amounts: readonly string[]): string {
  let total = 0n
  for (const amount of amounts) {
    total += toSatang(amount)
  }
  return satangToText(total)
}
