// Query keys and options for the Overview screen's month totals (ADR-0089).

import { queryOptions } from '@tanstack/react-query'

import { getDashboard } from '@/api/dashboard'

/**
 * The prefix of every dashboard month; invalidated with the transaction lists and the
 * categories (see `invalidateTransactions` and `invalidateCategories`), so Overview is never out
 * of date after an edit.
 */
export const dashboardKey = ['dashboard'] as const

/** One month's totals; `month` is written YYYY-MM. */
export function dashboardQuery(month: string) {
  return queryOptions({
    queryKey: [...dashboardKey, month],
    queryFn: ({ signal }) => getDashboard(month, signal),
  })
}
