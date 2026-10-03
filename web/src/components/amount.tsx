import { cn } from 'cn'

import { amountParts } from '@/lib/amount'

export interface AmountProps {
  /** The API's text form, such as "145.00"; never converted to a float. */
  value: string
  /** `income` is green with a `+`; an expense is plain ink (ADR-0072). */
  variant?: 'expense' | 'income'
  /** `row` for lists, `display` for a large single amount. */
  size?: 'row' | 'display'
  className?: string
}

const sizes = {
  row: { baht: 'text-base', satang: 'text-sm' },
  display: { baht: 'text-display', satang: 'text-xl' },
} as const

/**
 * Money as Satang writes it (ADR-0072): the baht part large and dark, the two satang digits
 * smaller and brass; tabular figures so amounts line up in a list.
 */
export function Amount({ value, variant = 'expense', size = 'row', className }: AmountProps) {
  const parts = amountParts(value)
  const income = variant === 'income'
  const classes = cn(
    'inline-flex items-baseline font-semibold whitespace-nowrap tabular-nums',
    income ? 'text-income' : 'text-foreground',
    className,
  )
  if (!parts) {
    // Not an amount: show the text as it came rather than inventing a number.
    return <span className={cn(classes, sizes[size].baht)}>{value}</span>
  }
  const sign = income ? '+' : parts.sign
  return (
    <span data-slot="amount" className={classes}>
      <span className={sizes[size].baht}>
        {sign}
        {parts.baht}
      </span>
      <span className={cn(sizes[size].satang, income ? 'text-income' : 'text-brass')}>
        .{parts.satang}
      </span>
    </span>
  )
}
