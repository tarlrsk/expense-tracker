import type { CategoryKind } from '@/api/types'

// The fields of a transaction, shared by Add and the transaction sheet in History.
export const transaction = {
  amount: 'Amount',
  kind: 'Type',
  expense: 'Expense',
  income: 'Income',
  category: 'Category',
  archived: 'Archived',
  date: 'Date',
  merchant: 'Merchant',
  merchantHint: 'Optional, up to 100 characters.',
  note: 'Note',
  noteHint: 'Optional, up to 500 characters.',
  noCategories: (kind: CategoryKind) =>
    kind === 'expense'
      ? 'You have no expense categories to choose from.'
      : 'You have no income categories to choose from.',
  toCategories: 'Add one in Categories',
  categoriesLoadError: 'Your categories could not be loaded.',
  unknownCategory: 'Unknown category',

  enterAmount: 'Enter an amount.',
  amountFormat: 'Use digits and a point, such as 145.50.',
  amountDecimals: 'Use at most two decimals.',
  amountZero: 'Enter an amount more than 0.',
  amountTooLarge: 'Use at most 9,999,999,999.99.',
  chooseCategory: 'Choose a category.',
  chooseActiveCategory: 'Choose one of your active categories.',
  dateRule: 'Choose a date from 1 Jan 2000 to one year from today.',
  merchantTooLong: 'Use at most 100 characters.',
  noteTooLong: 'Use at most 500 characters.',
  specialCharacters: 'Remove tabs and other special characters.',

  // The transaction sheet in History.
  saveChanges: 'Save changes',
  saving: 'Saving…',
  delete: 'Delete',
  deleteTitle: 'Delete this transaction?',
  deleteWarning: (amount: string, date: string) =>
    `The ${amount} on ${date} is removed for good. This cannot be undone.`,
  deleteSubmit: 'Delete',
  deleting: 'Deleting…',
  saved: 'Changes saved.',
  deleted: 'Transaction deleted.',
  gone: 'This transaction no longer exists. The list has been refreshed.',
} as const
