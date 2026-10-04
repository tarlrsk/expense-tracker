// The rules of a transaction (ADR-0071), checked before anything is sent so each error can be
// shown beside its field (ADR-0072). The API checks them again.

import type { Category, CategoryKind, Transaction, UpdateTransactionRequest } from '@/api/types'
import { transaction as t } from '@/messages/transaction'

import { isDate, maxTransactionDate, minTransactionDate } from './dates'
import { parseAmountInput } from './money'
import type { AmountInputError } from './money'
import { characterCount } from './password'

export const maxMerchantLength = 100
export const maxNoteLength = 500

/** What the form holds while the user types. */
export interface TransactionDraft {
  amount: string
  kind: CategoryKind
  categoryId: string | null
  date: string
  merchant: string
  note: string
}

export type TransactionField = 'amount' | 'category' | 'date' | 'merchant' | 'note'

export type FieldErrors = Partial<Record<TransactionField, string>>

/** A draft that passed every rule, normalised as the API stores it. */
export interface CheckedTransaction {
  amount: string
  category_id: string
  occurred_on: string
  merchant: string
  note: string
}

export type CheckResult =
  { ok: true; value: CheckedTransaction } | { ok: false; errors: FieldErrors }

// Control characters (Unicode Cc), as the API's unicode.IsControl.
const controlPattern = /\p{Cc}/u
const controlExceptNewline = /[^\P{Cc}\n]/u

const amountMessages: Record<AmountInputError, string> = {
  empty: t.enterAmount,
  format: t.amountFormat,
  decimals: t.amountDecimals,
  zero: t.amountZero,
  too_large: t.amountTooLarge,
}

/** The note as the API stores it: Windows line endings become plain ones, then trimmed. */
export function normalizeNote(note: string): string {
  return note.replaceAll('\r\n', '\n').trim()
}

/**
 * Checks a draft. `categoryIds` are the categories that may be chosen: the active ones of the
 * draft's kind, plus a transaction's own archived category when it is kept.
 */
export function checkTransaction(
  draft: TransactionDraft,
  { today, categoryIds }: { today: string; categoryIds: readonly string[] },
): CheckResult {
  const errors: FieldErrors = {}

  const amount = parseAmountInput(draft.amount)
  if (!amount.ok) {
    errors.amount = amountMessages[amount.error]
  }

  if (draft.categoryId === null) {
    errors.category = t.chooseCategory
  } else if (!categoryIds.includes(draft.categoryId)) {
    errors.category = t.chooseActiveCategory
  }

  if (
    !isDate(draft.date) ||
    draft.date < minTransactionDate ||
    draft.date > maxTransactionDate(today)
  ) {
    errors.date = t.dateRule
  }

  const merchant = draft.merchant.trim()
  if (characterCount(merchant) > maxMerchantLength) {
    errors.merchant = t.merchantTooLong
  } else if (controlPattern.test(merchant)) {
    errors.merchant = t.specialCharacters
  }

  const note = normalizeNote(draft.note)
  if (characterCount(note) > maxNoteLength) {
    errors.note = t.noteTooLong
  } else if (controlExceptNewline.test(note)) {
    errors.note = t.specialCharacters
  }

  if (!amount.ok || draft.categoryId === null || Object.keys(errors).length > 0) {
    return { ok: false, errors }
  }
  return {
    ok: true,
    value: {
      amount: amount.value,
      category_id: draft.categoryId,
      occurred_on: draft.date,
      merchant,
      note,
    },
  }
}

/**
 * The field an `invalid_input` from the API most likely belongs to, read from the start of
 * its message (error bodies carry no field name, ADR-0072, ADR-0074); null when unknown.
 */
export function fieldOfApiError(message: string): TransactionField | null {
  const m = message.trim().toLowerCase()
  if (m.startsWith('amount')) {
    return 'amount'
  }
  if (m.startsWith('occurred_on')) {
    return 'date'
  }
  if (m.startsWith('choose one of your active categories')) {
    return 'category'
  }
  if (m.startsWith('merchant')) {
    return 'merchant'
  }
  if (m.startsWith('note')) {
    return 'note'
  }
  return null
}

/** The app's own wording for a field the API refused. */
export const apiFieldMessages: Record<TransactionField, string> = {
  amount: t.amountFormat,
  category: t.chooseActiveCategory,
  date: t.dateRule,
  merchant: t.merchantTooLong,
  note: t.noteTooLong,
}

/**
 * The categories that may be chosen for `kind`: the active ones in the user's order, plus
 * `keep` (a transaction's own archived category, which may stay) at the end.
 */
export function categoryChoices(
  categories: readonly Category[],
  kind: CategoryKind,
  keep?: Category,
): Category[] {
  const choices = categories.filter((c) => c.kind === kind && !c.archived)
  if (keep?.archived && keep.kind === kind) {
    choices.push(keep)
  }
  return choices
}

/** Only the fields that differ from the stored transaction (ADR-0071: PATCH sends what changes). */
export function changedFields(
  stored: Transaction,
  checked: CheckedTransaction,
): UpdateTransactionRequest {
  const changes: UpdateTransactionRequest = {}
  if (checked.amount !== stored.amount) {
    changes.amount = checked.amount
  }
  if (checked.category_id !== stored.category_id) {
    changes.category_id = checked.category_id
  }
  if (checked.occurred_on !== stored.occurred_on) {
    changes.occurred_on = checked.occurred_on
  }
  if (checked.merchant !== stored.merchant) {
    changes.merchant = checked.merchant
  }
  if (checked.note !== stored.note) {
    changes.note = checked.note
  }
  return changes
}
