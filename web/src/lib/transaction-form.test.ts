import { describe, expect, it } from 'vitest'

import { checkTransaction, fieldOfApiError } from './transaction-form'
import type { TransactionDraft } from './transaction-form'

const today = '2026-10-04'
const dateRule = 'Choose a date from 1 Jan 2000 to one year from today.'
const special = 'Remove tabs and other special characters.'
const food = 'cat-food'

const good: TransactionDraft = {
  amount: '145',
  kind: 'expense',
  categoryId: food,
  date: today,
  merchant: '  Noodle shop ',
  note: 'line one\r\nline two  ',
}

function check(fields: Partial<TransactionDraft>) {
  return checkTransaction({ ...good, ...fields }, { today, categoryIds: [food] })
}

describe('checkTransaction', () => {
  it('normalises a good draft as the API stores it', () => {
    expect(check({})).toEqual({
      ok: true,
      value: {
        amount: '145.00',
        category_id: food,
        occurred_on: today,
        merchant: 'Noodle shop',
        note: 'line one\nline two',
      },
    })
  })

  it.each([
    { name: 'the earliest date', fields: { date: '2000-01-01' } },
    { name: 'one year ahead', fields: { date: '2027-10-04' } },
    { name: 'no merchant or note', fields: { merchant: '', note: '' } },
    { name: '100-character merchant', fields: { merchant: 'm'.repeat(100) } },
    { name: '500-character note', fields: { note: 'n'.repeat(500) } },
    { name: '100 emoji (counted as characters)', fields: { merchant: '🍜'.repeat(100) } },
  ])('takes $name', ({ fields }) => {
    expect(check(fields).ok).toBe(true)
  })

  it.each([
    { name: 'no amount', fields: { amount: '' }, field: 'amount', error: 'Enter an amount.' },
    {
      name: 'a zero amount',
      fields: { amount: '0' },
      field: 'amount',
      error: 'Enter an amount more than 0.',
    },
    {
      name: 'three decimals',
      fields: { amount: '1.005' },
      field: 'amount',
      error: 'Use at most two decimals.',
    },
    {
      name: 'no category',
      fields: { categoryId: null },
      field: 'category',
      error: 'Choose a category.',
    },
    {
      name: 'a category not offered',
      fields: { categoryId: 'archived' },
      field: 'category',
      error: 'Choose one of your active categories.',
    },
    { name: 'a date before 2000', fields: { date: '1999-12-31' }, field: 'date', error: dateRule },
    {
      name: 'a date too far ahead',
      fields: { date: '2027-10-05' },
      field: 'date',
      error: dateRule,
    },
    { name: 'no date', fields: { date: '' }, field: 'date', error: dateRule },
    {
      name: 'a long merchant',
      fields: { merchant: 'm'.repeat(101) },
      field: 'merchant',
      error: 'Use at most 100 characters.',
    },
    {
      name: 'a tab in the merchant',
      fields: { merchant: 'a\tb' },
      field: 'merchant',
      error: special,
    },
    {
      name: 'a long note',
      fields: { note: 'n'.repeat(501) },
      field: 'note',
      error: 'Use at most 500 characters.',
    },
    {
      name: 'a control character in the note',
      fields: { note: 'a\u0007b' },
      field: 'note',
      error: special,
    },
  ])('refuses $name', ({ fields, field, error }) => {
    const result = check(fields)
    expect(result.ok).toBe(false)
    expect(result.ok ? {} : result.errors).toEqual({ [field]: error })
  })

  it('reports every broken field at once', () => {
    const result = check({ amount: '', categoryId: null, date: '' })
    expect(result.ok ? [] : Object.keys(result.errors).sort()).toEqual([
      'amount',
      'category',
      'date',
    ])
  })
})

describe('fieldOfApiError', () => {
  it.each([
    { message: 'amount must be text such as "145.00"', field: 'amount' },
    { message: 'occurred_on must be a real date written YYYY-MM-DD', field: 'date' },
    {
      message: 'choose one of your active categories (create or unarchive one)',
      field: 'category',
    },
    { message: 'merchant must be at most 100 characters', field: 'merchant' },
    { message: 'note must be at most 500 characters', field: 'note' },
    { message: 'id must be a new UUID version 7', field: null },
    { message: 'the request body is not valid JSON', field: null },
  ])('$message → $field', ({ message, field }) => {
    expect(fieldOfApiError(message)).toBe(field)
  })
})
