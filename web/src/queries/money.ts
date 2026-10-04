// Query keys and options for categories and transactions, shared by every screen so a change on
// one screen refreshes the others.

import { queryOptions } from '@tanstack/react-query'

import { listCategories } from '@/api/categories'
import { listAllTransactions } from '@/api/transactions'
import type { TransactionPeriod } from '@/api/types'
import { monthParam } from '@/lib/dates'
import type { Period } from '@/lib/dates'

export const categoriesQuery = queryOptions({
  queryKey: ['categories'],
  queryFn: ({ signal }) => listCategories(signal),
})

/** The prefix of every transaction list; invalidate it after any transaction changes. */
export const transactionsKey = ['transactions'] as const

/** A week as `from` / `to`, a month as `month` (ADR-0071). */
export function periodFilter(period: Period): TransactionPeriod {
  return period.view === 'month'
    ? { month: monthParam(period) }
    : { from: period.from, to: period.to }
}

/** Every transaction of a period, all pages loaded (ADR-0074). */
export function periodTransactionsQuery(period: Period) {
  const filter = periodFilter(period)
  return queryOptions({
    queryKey: [...transactionsKey, 'period', filter],
    queryFn: ({ signal }) => listAllTransactions(filter, signal),
  })
}
