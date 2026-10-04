// Quick entry (ADR-0076, ADR-0082): the typed text is checked before it is sent, each proposal
// the API returns becomes a draft of the manual form, and nothing is saved until the user
// confirms. These are the pure parts; the screen is in components/quick-entry.tsx.

import type { Category, CreateTransactionRequest, EntryItem } from '@/api/types'
import { entry as t } from '@/messages/entry'

import { characterCount } from './password'
import { categoryChoices, checkTransaction } from './transaction-form'
import type {
  CheckedTransaction,
  CheckResult,
  FieldErrors,
  TransactionDraft,
} from './transaction-form'
import { uuidv7 } from './uuid'

/** The API's limit on the text, in characters once trimmed (ADR-0082). */
export const maxEntryTextLength = 1000

export type EntryTextResult = { ok: true; text: string } | { ok: false; error: string }

/** Checks the typed text as the API will: 1 to 1,000 characters once trimmed. */
export function checkEntryText(text: string): EntryTextResult {
  const trimmed = text.trim()
  if (trimmed === '') {
    return { ok: false, error: t.enterText }
  }
  if (characterCount(trimmed) > maxEntryTextLength) {
    return { ok: false, error: t.textTooLong }
  }
  return { ok: true, text: trimmed }
}

/**
 * The app's wording for an `invalid_input` from POST /api/entry/parse that belongs beside the
 * text box, read from the start of its message (error bodies carry no field name); null when
 * the message is not one of these.
 */
export function entryTextErrorOfApi(message: string): string | null {
  const m = message.trim().toLowerCase()
  if (m.startsWith('text must hold at least one item')) {
    return t.noItem
  }
  if (m.startsWith('text must hold at most')) {
    return t.tooManyItems
  }
  if (m.startsWith('text must be')) {
    return t.textRule
  }
  return null
}

/** One proposal in the confirm sheet. */
export interface Proposal {
  /**
   * The transaction id (UUID v7), made once when the sheet opens, so sending it again after a
   * failure saves it once (ADR-0040).
   */
  id: string
  /** The item as typed; sent as `raw_input`, whatever the user changes. */
  text: string
  draft: TransactionDraft
  /** The parser or the AI was unsure; marked "Check this" until the user changes the row. */
  unsure: boolean
}

/**
 * A proposal from the API as a draft of the manual form. Its category is kept only when it is
 * one of the user's active categories, and sets the kind (expense when there is none). A date
 * that could not be read becomes `today`.
 */
export function toProposal(
  item: EntryItem,
  categories: readonly Category[],
  today: string,
  id: string = uuidv7(),
): Proposal {
  const category = categories.find((c) => c.id === item.category_id && !c.archived)
  return {
    id,
    text: item.text,
    draft: {
      amount: item.amount,
      kind: category?.kind ?? 'expense',
      categoryId: category?.id ?? null,
      date: item.occurred_on === '' ? today : item.occurred_on,
      merchant: item.merchant,
      note: '',
    },
    unsure: item.confidence === 'low',
  }
}

/** Checks a proposal as the manual form does: only active categories of its kind may be chosen. */
export function checkProposal(
  proposal: Proposal,
  categories: readonly Category[],
  today: string,
): CheckResult {
  const choices = categoryChoices(categories, proposal.draft.kind)
  return checkTransaction(proposal.draft, { today, categoryIds: choices.map((c) => c.id) })
}

export interface ProposalSplit {
  ready: { proposal: Proposal; value: CheckedTransaction }[]
  incomplete: { proposal: Proposal; errors: FieldErrors }[]
}

/** Splits proposals into those that can be saved now and those that still break a rule, in order. */
export function splitProposals(
  proposals: readonly Proposal[],
  categories: readonly Category[],
  today: string,
): ProposalSplit {
  const split: ProposalSplit = { ready: [], incomplete: [] }
  for (const proposal of proposals) {
    const result = checkProposal(proposal, categories, today)
    if (result.ok) {
      split.ready.push({ proposal, value: result.value })
    } else {
      split.incomplete.push({ proposal, errors: result.errors })
    }
  }
  return split
}

/**
 * The body that saves a checked proposal: `source: "text"` with the typed item as `raw_input`;
 * an empty description or note is left out, as Add does.
 */
export function textTransactionRequest(
  proposal: Proposal,
  value: CheckedTransaction,
): CreateTransactionRequest {
  const { merchant, note, ...required } = value
  const req: CreateTransactionRequest = {
    id: proposal.id,
    ...required,
    source: 'text',
    raw_input: proposal.text,
  }
  if (merchant !== '') {
    req.merchant = merchant
  }
  if (note !== '') {
    req.note = note
  }
  return req
}

/** What a row is called: its description, else its category's name, else the text as typed. */
export function proposalTitle(proposal: Proposal, category: Category | undefined): string {
  const merchant = proposal.draft.merchant.trim()
  if (merchant !== '') {
    return merchant
  }
  if (category) {
    return category.name
  }
  const text = proposal.text.trim()
  return text === '' ? t.noDescription : text
}

/** The notice once the sheet closes: how many were saved; null when none was. */
export function savedNotice(saved: number, left: number): string | null {
  if (saved === 0) {
    return null
  }
  return left === 0 ? t.saved(saved) : t.savedSome(saved)
}
