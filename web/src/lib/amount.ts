// Money is text from the API, such as "145.00" (ADR-0071); it is never turned into a float.

export interface AmountParts {
  /** "-" for a negative amount, else "". */
  sign: '' | '-'
  /** Whole baht with thousands separators, e.g. "1,234". */
  baht: string
  /** The two satang digits, e.g. "05". */
  satang: string
}

const amountPattern = /^(-?)(\d+)\.(\d{2})$/

/** Splits "1234.05" into its display parts, or returns null when the text is not an amount. */
export function amountParts(text: string): AmountParts | null {
  const match = amountPattern.exec(text.trim())
  if (!match) {
    return null
  }
  const [, sign = '', whole = '', satang = ''] = match
  const baht = whole.replace(/^0+(?=\d)/, '').replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return { sign: sign === '-' ? '-' : '', baht, satang }
}
