import type { EntryAIStatus } from '@/api/types'

const plural = (n: number, one: string, many: string) => (n === 1 ? one : many)

// Quick entry on the Add screen and its confirm sheet (ADR-0076, ADR-0082).
export const entry = {
  mode: 'How to add',
  quick: 'Quick',
  form: 'Form',

  textLabel: 'Entries',
  textHint:
    'Type one or more, such as “coffee 60, grab 145 yesterday”. Thai works too. Up to 1,000 characters.',
  read: 'Read entries',
  reading: 'Reading…',

  enterText: 'Type at least one entry, such as “coffee 60”.',
  textTooLong: 'Use at most 1,000 characters.',
  textRule: 'Type 1 to 1,000 characters.',
  noItem: 'Type at least one entry with what it was, such as “coffee 60”.',
  tooManyItems: 'Use at most 20 entries at a time.',

  sheetTitle: 'Check and save',
  sheetDescription: (n: number) =>
    plural(n, '1 entry. Tap it to change it.', `${String(n)} entries. Tap one to change it.`),
  /** Said at the top of the sheet when the AI did not help; null when it was used or not needed. */
  aiNotice: (ai: EntryAIStatus): string | null => {
    switch (ai) {
      case 'limit_reached':
        return "Today's AI limit is reached. Choose the categories yourself."
      case 'unavailable':
        return 'The AI could not be reached. Choose the categories yourself.'
      case 'not_configured':
        return 'The AI is not set up. Choose the categories yourself.'
      case 'not_needed':
      case 'used':
        return null
    }
  },
  checkThis: 'Check this',
  noDescription: 'No description',
  remove: (name: string) => `Remove ${name}`,
  done: 'Done',
  saveAll: 'Save all',
  saving: 'Saving…',

  someIncomplete: (n: number) =>
    plural(n, '1 entry is not finished yet.', `${String(n)} entries are not finished yet.`),
  saveReady: (n: number) => plural(n, 'Save the ready one', `Save the ${String(n)} ready ones`),
  finishFirst: 'Finish the rest first',

  saved: (n: number) => plural(n, 'Saved 1 transaction.', `Saved ${String(n)} transactions.`),
  savedSome: (n: number) =>
    `${plural(n, 'Saved 1 transaction.', `Saved ${String(n)} transactions.`)} The rest were not saved.`,
} as const
