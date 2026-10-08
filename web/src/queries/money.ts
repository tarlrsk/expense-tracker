// Query keys and options for categories and transactions, shared by every screen so a change on
// one screen refreshes the others.

import { queryOptions } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'

import { listCategories } from '@/api/categories'
import { listAllTransactions } from '@/api/transactions'
import type { TransactionPeriod } from '@/api/types'
import { monthParam } from '@/lib/dates'
import type { Period } from '@/lib/dates'

import { dashboardKey } from './dashboard'

export const categoriesQuery = queryOptions({
  queryKey: ['categories'],
  queryFn: ({ signal }) => listCategories(signal),
})

/** The prefix of every transaction list; use `invalidateTransactions` after any change. */
export const transactionsKey = ['transactions'] as const

/** After a transaction is saved, changed or deleted: every list and every dashboard month. */
export async function invalidateTransactions(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: transactionsKey }),
    queryClient.invalidateQueries({ queryKey: dashboardKey }),
  ])
}

/**
 * After a category changes (name, icon, archived, order): the list and every dashboard month,
 * whose rows carry the category's current name, icon and flag.
 */
export async function invalidateCategories(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: categoriesQuery.queryKey }),
    queryClient.invalidateQueries({ queryKey: dashboardKey }),
  ])
}

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
