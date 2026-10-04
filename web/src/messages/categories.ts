import type { CategoryKind } from '@/api/types'

export const categories = {
  title: 'Categories',
  intro: 'Choose them when you add an expense or income. Archived ones stay on what you recorded.',
  expenses: 'Expenses',
  income: 'Income',
  archived: 'Archived',
  noneActive: (kind: CategoryKind) =>
    kind === 'expense' ? 'No expense categories.' : 'No income categories.',
  loadError: 'Your categories could not be loaded.',

  add: 'Add category',
  editOrder: 'Edit order',
  saveOrder: 'Save order',
  savingOrder: 'Saving…',
  orderSaved: 'Order saved.',
  moveUp: (name: string) => `Move ${name} up`,
  moveDown: (name: string) => `Move ${name} down`,

  addTitle: 'Add category',
  name: 'Name',
  nameHint: '1 to 50 characters.',
  kind: 'Type',
  expense: 'Expense',
  incomeKind: 'Income',
  icon: 'Icon',
  iconHint: 'Optional. Type one emoji, such as 🍜.',
  create: 'Add category',
  creating: 'Adding…',
  created: (name: string) => `${name} added.`,

  editTitle: 'Edit category',
  kindFixed: (kind: CategoryKind) =>
    kind === 'expense'
      ? 'Expense category. The type cannot be changed.'
      : 'Income category. The type cannot be changed.',
  save: 'Save',
  saving: 'Saving…',
  saved: (name: string) => `${name} saved.`,
  archive: 'Archive',
  archiving: 'Archiving…',
  archivedOk: (name: string) =>
    `${name} archived. It is no longer offered, and stays on what you recorded.`,
  unarchive: 'Unarchive',
  unarchiving: 'Unarchiving…',
  unarchivedOk: (name: string) => `${name} is active again, at the end of its list.`,

  enterName: 'Enter a name.',
  nameTooLong: 'Use at most 50 characters.',
  iconTooLong: 'Use at most 32 characters.',
  specialCharacters: 'Remove tabs and other special characters.',
  limitReached: 'You have 200 categories, the most there can be. Archived ones count too.',
} as const
