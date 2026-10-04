// The user's transactions: /api/transactions (docs/04-api.md, ADR-0040, ADR-0071). Amounts
// stay text; nothing here turns one into a number.

import { ApiError, apiRequest } from './client'
import {
  amount,
  array,
  date,
  nonEmptyString,
  nullableString,
  object,
  oneOf,
  string,
  timestamp,
} from './shape'
import { transactionSources } from './types'
import type {
  CreateTransactionRequest,
  ListTransactionsParams,
  ListTransactionsResponse,
  Transaction,
  TransactionPeriod,
  UpdateTransactionRequest,
} from './types'

export function parseTransaction(value: unknown): Transaction {
  const o = object(value, 'transaction')
  return {
    id: nonEmptyString(o, 'id'),
    owner_id: nonEmptyString(o, 'owner_id'),
    amount: amount(o, 'amount'),
    currency: string(o, 'currency'),
    occurred_on: date(o, 'occurred_on'),
    merchant: string(o, 'merchant'),
    category_id: nonEmptyString(o, 'category_id'),
    note: string(o, 'note'),
    source: oneOf(o, 'source', transactionSources),
    created_at: timestamp(o, 'created_at'),
    updated_at: timestamp(o, 'updated_at'),
  }
}

export function parseListTransactions(body: unknown): ListTransactionsResponse {
  const o = object(body, 'list')
  return {
    transactions: array(o.transactions, 'transactions').map(parseTransaction),
    next_cursor: nullableString(o, 'next_cursor'),
  }
}

/** The query string of a list; parameters that are not set are left out (the API refuses empty ones). */
function listQuery(params: ListTransactionsParams): string {
  const q = new URLSearchParams()
  if ('month' in params) {
    q.set('month', params.month)
  } else {
    if (params.from !== undefined) {
      q.set('from', params.from)
    }
    if (params.to !== undefined) {
      q.set('to', params.to)
    }
  }
  if (params.limit !== undefined) {
    q.set('limit', String(params.limit))
  }
  if (params.cursor !== undefined) {
    q.set('cursor', params.cursor)
  }
  const text = q.toString()
  return text === '' ? '' : `?${text}`
}

/** GET /api/transactions — one page, newest first. */
export function listTransactions(
  params: ListTransactionsParams,
  signal?: AbortSignal,
): Promise<ListTransactionsResponse> {
  return apiRequest(`/transactions${listQuery(params)}`, { signal, parse: parseListTransactions })
}

/** The largest page the API gives (ADR-0071): fewest round trips for a whole period. */
export const maxPageSize = 200

/**
 * Every transaction of a period, following `next_cursor` until the last page, newest first. A
 * whole period is needed for its day totals to be right (ADR-0074).
 */
export async function listAllTransactions(
  period: TransactionPeriod,
  signal?: AbortSignal,
): Promise<Transaction[]> {
  const all: Transaction[] = []
  const seen = new Set<string>()
  let cursor: string | undefined
  for (;;) {
    const page = await listTransactions({ ...period, limit: maxPageSize, cursor }, signal)
    all.push(...page.transactions)
    if (page.next_cursor === null) {
      return all
    }
    if (seen.has(page.next_cursor)) {
      // A cursor that comes back again would never end; treat it as a broken answer.
      throw new ApiError(200, 'bad_response', 'Unexpected response from the server (status 200).')
    }
    seen.add(page.next_cursor)
    cursor = page.next_cursor
  }
}

/**
 * POST /api/transactions — 201 the first time; the same id again returns the stored
 * transaction with 200, so a retry never saves twice (ADR-0040).
 */
export function createTransaction(req: CreateTransactionRequest): Promise<Transaction> {
  return apiRequest('/transactions', { method: 'POST', body: req, parse: parseTransaction })
}

/** PATCH /api/transactions/{id} — only the fields sent change; a new category must be active. */
export function updateTransaction(id: string, req: UpdateTransactionRequest): Promise<Transaction> {
  return apiRequest(`/transactions/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: req,
    parse: parseTransaction,
  })
}

/** DELETE /api/transactions/{id} — 404 `not_found` for an unknown or another user's id. */
export function deleteTransaction(id: string): Promise<void> {
  return apiRequest<undefined>(`/transactions/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
