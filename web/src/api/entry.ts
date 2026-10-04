// Quick entry: POST /api/entry/parse (docs/04-api.md, ADR-0082). It only proposes; nothing is
// saved until each item is sent to POST /api/transactions. Amounts stay text.

import { apiRequest, ResponseShapeError } from './client'
import { array, nullableString, object, oneOf, string } from './shape'
import type { JsonObject } from './shape'
import { entryAIStatuses, entryConfidences, entryResolvers } from './types'
import type { EntryItem, ParseEntryRequest, ParseEntryResponse } from './types'

/** Text that is either `""` (not read) or matches `pattern`. */
function emptyOr(obj: JsonObject, key: string, pattern: RegExp, what: string): string {
  const value = string(obj, key)
  if (value !== '' && !pattern.test(value)) {
    throw new ResponseShapeError(`${key} is not ${what}`)
  }
  return value
}

function parseEntryItem(value: unknown): EntryItem {
  const o = object(value, 'item')
  const categoryId = nullableString(o, 'category_id')
  if (categoryId === '') {
    throw new ResponseShapeError('category_id is empty')
  }
  return {
    text: string(o, 'text'),
    amount: emptyOr(o, 'amount', /^\d+\.\d{2}$/, 'an amount'),
    occurred_on: emptyOr(o, 'occurred_on', /^\d{4}-\d{2}-\d{2}$/, 'a date'),
    merchant: string(o, 'merchant'),
    category_id: categoryId,
    confidence: oneOf(o, 'confidence', entryConfidences),
    resolved_by: oneOf(o, 'resolved_by', entryResolvers),
  }
}

export function parseParseEntry(body: unknown): ParseEntryResponse {
  const o = object(body, 'parse')
  return {
    items: array(o.items, 'items').map(parseEntryItem),
    ai: oneOf(o, 'ai', entryAIStatuses),
  }
}

/**
 * POST /api/entry/parse — the proposals for a typed text, in the order typed. Without the AI
 * (limit reached, down, not set up) the items still come back, with `ai` saying why.
 */
export function parseEntry(text: string, signal?: AbortSignal): Promise<ParseEntryResponse> {
  const body: ParseEntryRequest = { text }
  return apiRequest('/entry/parse', { method: 'POST', body, signal, parse: parseParseEntry })
}
