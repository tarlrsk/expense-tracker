// Shares of a month's spending for the Overview breakdown (ADR-0089: the API sends totals, the
// screen works out the percentages). Amounts are text such as "145.00" and are compared in whole
// satang with BigInt: never as floats.

import { toSatang } from './money'

/**
 * `part` as a share of `whole`, written as a whole percent rounded half up: "42%". A share above
 * zero that rounds to 0 is "<1%", so a small category never reads as nothing. Null when `whole`
 * is not above zero (there is nothing to share).
 */
export function shareLabel(part: string, whole: string): string | null {
  const p = toSatang(part)
  const w = toSatang(whole)
  if (w <= 0n) {
    return null
  }
  if (p <= 0n) {
    return '0%'
  }
  // round(p * 100 / w), half up, in whole numbers.
  const percent = (p * 200n + w) / (2n * w)
  return percent === 0n ? '<1%' : `${percent.toString()}%`
}

/**
 * The width of a bar for `part` next to the largest value `max`, as a percent with two decimals
 * (0 to 100), for a CSS width. 0 when either is not above zero.
 */
export function barPercent(part: string, max: string): number {
  const p = toSatang(part)
  const m = toSatang(max)
  if (m <= 0n || p <= 0n) {
    return 0
  }
  const hundredths = (p * 10_000n) / m
  return Number(hundredths > 10_000n ? 10_000n : hundredths) / 100
}
