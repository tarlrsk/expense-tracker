import type { HistoryView } from '@/lib/dates'

export const history = {
  title: 'History',
  view: 'Show',
  week: 'Week',
  month: 'Month',
  previous: (view: HistoryView) => (view === 'week' ? 'Previous week' : 'Previous month'),
  next: (view: HistoryView) => (view === 'week' ? 'Next week' : 'Next month'),
  empty: (period: string) => `Nothing recorded for ${period}.`,
  emptyNote: 'What you add shows up here, grouped by day.',
  toAdd: 'Add an expense or income',
  loadError: 'Your history could not be loaded.',
  spent: 'Spent',
  received: 'Received',
} as const
