import { describe, expect, it } from 'vitest'

import type { Category, EntryItem } from '@/api/types'
import { category } from '@/test/api'

import {
  checkEntryText,
  entryTextErrorOfApi,
  proposalTitle,
  savedNotice,
  splitProposals,
  textTransactionRequest,
  toProposal,
} from './quick-entry'
import type { Proposal } from './quick-entry'

const today = '2026-10-04'
const food: Category = category(1, { name: 'Food', icon: '🍜' })
const salary: Category = category(2, { name: 'Salary', icon: '💼', kind: 'income' })
const old: Category = category(3, { name: 'Old', archived: true })
const categories = [food, salary, old]

const item: EntryItem = {
  text: 'coffee 60',
  amount: '60.00',
  occurred_on: '2026-10-03',
  merchant: 'coffee',
  category_id: food.id,
  confidence: 'high',
  resolved_by: 'rule',
}

function proposal(fields: Partial<EntryItem> = {}, id = 'id-1'): Proposal {
  return toProposal({ ...item, ...fields }, categories, today, id)
}

describe('checkEntryText', () => {
  it.each([
    { text: '', error: 'Type at least one entry, such as “coffee 60”.' },
    { text: ' \n\t ', error: 'Type at least one entry, such as “coffee 60”.' },
    { text: 'a'.repeat(1001), error: 'Use at most 1,000 characters.' },
    { text: '🍜'.repeat(1001), error: 'Use at most 1,000 characters.' },
  ])('refuses $text.length characters of that text', ({ text, error }) => {
    expect(checkEntryText(text)).toEqual({ ok: false, error })
  })

  it.each([
    { text: '  coffee 60 \n', want: 'coffee 60' },
    // 1,000 characters once trimmed; emoji count as one each, as the API counts runes.
    { text: ` ${'a'.repeat(1000)} `, want: 'a'.repeat(1000) },
    { text: '🍜'.repeat(1000), want: '🍜'.repeat(1000) },
  ])('takes and trims a text of $want.length units', ({ text, want }) => {
    expect(checkEntryText(text)).toEqual({ ok: true, text: want })
  })
})

describe('entryTextErrorOfApi', () => {
  it.each([
    {
      message: 'text must hold at least one item, such as "coffee 60"',
      want: 'Type at least one entry with what it was, such as “coffee 60”.',
    },
    { message: 'text must hold at most 20 items', want: 'Use at most 20 entries at a time.' },
    { message: 'text must be 1 to 1,000 characters', want: 'Type 1 to 1,000 characters.' },
    { message: 'request body is not valid JSON', want: null },
  ])('maps "$message"', ({ message, want }) => {
    expect(entryTextErrorOfApi(message)).toBe(want)
  })
})

describe('toProposal', () => {
  it('turns a complete item into a draft with its category and kind', () => {
    expect(proposal()).toEqual({
      id: 'id-1',
      text: 'coffee 60',
      draft: {
        amount: '60.00',
        kind: 'expense',
        categoryId: food.id,
        date: '2026-10-03',
        merchant: 'coffee',
        note: '',
      },
      unsure: false,
    })
  })

  it('takes the kind from an income category', () => {
    expect(proposal({ category_id: salary.id }).draft).toMatchObject({
      kind: 'income',
      categoryId: salary.id,
    })
  })

  it.each([
    { name: 'no category', categoryId: null },
    { name: 'an archived category', categoryId: old.id },
    { name: 'a category the user does not have', categoryId: 'someone-else' },
  ])('treats $name as none, as an expense', ({ categoryId }) => {
    expect(proposal({ category_id: categoryId }).draft).toMatchObject({
      kind: 'expense',
      categoryId: null,
    })
  })

  it('keeps unread fields empty, dates them today, and marks low confidence', () => {
    const p = proposal({ amount: '', occurred_on: '', merchant: '', confidence: 'low' })
    expect(p.draft).toMatchObject({ amount: '', date: today, merchant: '' })
    expect(p.unsure).toBe(true)
  })

  it('makes a UUID v7 when no id is given', () => {
    const p = toProposal(item, categories, today)
    expect(p.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7/)
    expect(toProposal(item, categories, today).id).not.toBe(p.id)
  })
})

describe('splitProposals', () => {
  it('keeps the order and tells ready from incomplete with the field errors', () => {
    const a = proposal({}, 'a')
    const b = proposal({ category_id: null }, 'b')
    const c = proposal({ amount: '' }, 'c')
    const d = proposal({ category_id: salary.id }, 'd')

    const split = splitProposals([a, b, c, d], categories, today)

    expect(split.ready.map((r) => r.proposal.id)).toEqual(['a', 'd'])
    expect(split.ready[0]?.value).toEqual({
      amount: '60.00',
      category_id: food.id,
      occurred_on: '2026-10-03',
      merchant: 'coffee',
      note: '',
    })
    expect(split.incomplete).toEqual([
      { proposal: b, errors: { category: 'Choose a category.' } },
      { proposal: c, errors: { amount: 'Enter an amount.' } },
    ])
  })

  it('treats a category archived since the sheet opened as incomplete', () => {
    const p = proposal()
    const split = splitProposals([p], [{ ...food, archived: true }], today)
    expect(split.incomplete[0]?.errors).toEqual({
      category: 'Choose one of your active categories.',
    })
  })
})

describe('textTransactionRequest', () => {
  it('sends source text and the item as typed, leaving out an empty description and note', () => {
    const p = proposal({ text: 'กาแฟ 60' })
    expect(
      textTransactionRequest(p, {
        amount: '60.00',
        category_id: food.id,
        occurred_on: today,
        merchant: '',
        note: '',
      }),
    ).toEqual({
      id: 'id-1',
      amount: '60.00',
      category_id: food.id,
      occurred_on: today,
      source: 'text',
      raw_input: 'กาแฟ 60',
    })
  })

  it('keeps the raw input even when the description was changed', () => {
    const p = proposal()
    expect(
      textTransactionRequest(p, {
        amount: '65.00',
        category_id: food.id,
        occurred_on: today,
        merchant: 'Latte',
        note: 'oat milk',
      }),
    ).toMatchObject({ raw_input: 'coffee 60', merchant: 'Latte', note: 'oat milk' })
  })
})

describe('proposalTitle', () => {
  it.each([
    { name: 'the description', fields: {}, cat: food, want: 'coffee' },
    { name: 'the category name', fields: { merchant: ' ' }, cat: food, want: 'Food' },
    { name: 'the typed text', fields: { merchant: '' }, cat: undefined, want: 'coffee 60' },
  ])('is $name', ({ fields, cat, want }) => {
    expect(proposalTitle(proposal(fields), cat)).toBe(want)
  })
})

describe('savedNotice', () => {
  it.each([
    { saved: 0, left: 2, want: null },
    { saved: 1, left: 0, want: 'Saved 1 transaction.' },
    { saved: 2, left: 0, want: 'Saved 2 transactions.' },
    { saved: 2, left: 1, want: 'Saved 2 transactions. The rest were not saved.' },
  ])('says $want for $saved saved, $left left', ({ saved, left, want }) => {
    expect(savedNotice(saved, left)).toBe(want)
  })
})
