import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { apiError, category, json, mockApi, profile, transaction } from '@/test/api'
import type { RecordedCall } from '@/test/api'
import { renderApp } from '@/test/render'

const food = category(1, { name: 'Food', icon: '🍜' })
const transport = category(2, { name: 'Transport', icon: '🚌' })
const salary = category(3, { name: 'Salary', icon: '💼', kind: 'income' })
const old = category(4, { name: 'Old', icon: '📦', archived: true })

const v7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

interface Item {
  text: string
  amount: string
  occurred_on: string
  merchant: string
  category_id: string | null
  confidence: 'high' | 'low'
  resolved_by: 'rule' | 'ai' | 'none'
}

const coffee: Item = {
  text: 'coffee 60',
  amount: '60.00',
  occurred_on: '2026-10-04',
  merchant: 'coffee',
  category_id: food.id,
  confidence: 'high',
  resolved_by: 'rule',
}

const grab: Item = {
  text: 'grab 145 yesterday',
  amount: '145.00',
  occurred_on: '2026-10-03',
  merchant: 'grab',
  category_id: transport.id,
  confidence: 'low',
  resolved_by: 'ai',
}

/** An item without a category, as at the AI limit. */
const gift: Item = {
  text: 'gift 500',
  amount: '500.00',
  occurred_on: '2026-10-04',
  merchant: 'gift',
  category_id: null,
  confidence: 'low',
  resolved_by: 'none',
}

let api: ReturnType<typeof mockApi>

function parsed(items: Item[], ai = 'used') {
  api.on('POST /api/entry/parse', () => json(200, { items, ai }))
}

function savedBody(call: RecordedCall) {
  return json(201, { ...transaction(1), ...(call.body as object) })
}

beforeEach(() => {
  // Noon on Sunday 4 October 2026 in Bangkok.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-04T05:00:00Z'))
  api = mockApi({
    'GET /api/me': () => json(200, profile),
    'GET /api/categories': () => json(200, { categories: [food, transport, salary, old] }),
    'POST /api/transactions': savedBody,
    'GET /api/transactions': () => json(200, { transactions: [], next_cursor: null }),
  })
  parsed([coffee, grab])
})

afterEach(() => {
  vi.useRealTimers()
})

function openAdd() {
  return renderApp('/add', { token: 'tok' })
}

/** Opens Add, types `text` and reads it; returns the confirm sheet. */
async function readText(text = 'coffee 60, grab 145 yesterday') {
  const view = openAdd()
  await view.user.click(await screen.findByLabelText('Entries'))
  await view.user.paste(text)
  await view.user.click(screen.getByRole('button', { name: 'Read entries' }))
  const sheet = await screen.findByRole('dialog', { name: 'Check and save' })
  return { ...view, sheet }
}

/** The row header of a proposal, by the start of its title. */
function row(sheet: HTMLElement, title: string) {
  return within(sheet).getByRole('button', { name: new RegExp(`^${title}`) })
}

function posted() {
  return api.callsTo('POST /api/transactions').map((c) => c.body as Record<string, string>)
}

describe('Add modes', () => {
  it('opens on Quick and remembers Form on this device', async () => {
    const first = openAdd()

    expect(await screen.findByRole('radio', { name: 'Quick' })).toBeChecked()
    const box = screen.getByLabelText('Entries')
    expect(box.tagName).toBe('TEXTAREA')
    expect(box).toHaveAttribute('enterkeyhint', 'enter')
    expect(box).toHaveAccessibleDescription(
      expect.stringContaining('coffee 60, grab 145 yesterday'),
    )
    expect(screen.queryByLabelText('Amount')).toBeNull()

    await first.user.click(screen.getByRole('radio', { name: 'Form' }))
    expect(await screen.findByLabelText('Amount')).toBeInTheDocument()
    expect(localStorage.getItem('satang.add-mode')).toBe('form')
    first.unmount()

    openAdd()
    expect(await screen.findByRole('radio', { name: 'Form' })).toBeChecked()
    expect(await screen.findByLabelText('Amount')).toBeInTheDocument()
  })
})

describe('Add, Quick mode', () => {
  it.each([
    { name: 'empty', text: '   ', error: 'Type at least one entry, such as “coffee 60”.' },
    { name: 'too long', text: 'a'.repeat(1001), error: 'Use at most 1,000 characters.' },
  ])('checks $name text beside the box before sending', async ({ text, error }) => {
    const { user } = openAdd()

    const box = await screen.findByLabelText('Entries')
    await user.click(box)
    await user.paste(text)
    await user.click(screen.getByRole('button', { name: 'Read entries' }))

    expect(box).toHaveAccessibleDescription(expect.stringContaining(error))
    expect(box).toHaveAttribute('aria-invalid', 'true')
    expect(api.callsTo('POST /api/entry/parse')).toHaveLength(0)
  })

  it('sends the trimmed text once and shows a pending label', async () => {
    let answer: (r: Response) => void = () => undefined
    api.on(
      'POST /api/entry/parse',
      () =>
        new Promise<Response>((resolve) => {
          answer = resolve
        }),
    )
    const { user } = openAdd()

    await user.click(await screen.findByLabelText('Entries'))
    await user.paste('  coffee 60\n')
    await user.click(screen.getByRole('button', { name: 'Read entries' }))
    const pending = await screen.findByRole('button', { name: 'Reading…' })
    expect(pending).toHaveAttribute('aria-disabled', 'true')
    await user.click(pending)

    expect(api.onlyCall('POST /api/entry/parse').body).toEqual({ text: 'coffee 60' })
    answer(json(200, { items: [coffee], ai: 'not_needed' }))
    expect(await screen.findByRole('dialog', { name: 'Check and save' })).toBeInTheDocument()
  })

  it.each([
    {
      message: 'text must hold at most 20 items',
      error: 'Use at most 20 entries at a time.',
    },
    {
      message: 'text must hold at least one item, such as "coffee 60"',
      error: 'Type at least one entry with what it was, such as “coffee 60”.',
    },
  ])('shows the API refusal "$message" beside the box', async ({ message, error }) => {
    api.on('POST /api/entry/parse', () => apiError(400, 'invalid_input', message))
    const { user } = openAdd()

    const box = await screen.findByLabelText('Entries')
    await user.click(box)
    await user.paste('yesterday')
    await user.click(screen.getByRole('button', { name: 'Read entries' }))

    await waitFor(() => {
      expect(box).toHaveAccessibleDescription(expect.stringContaining(error))
    })
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('shows any other parse failure under the box and keeps the text', async () => {
    api.on('POST /api/entry/parse', () => apiError(500, 'internal', 'internal error'))
    const { user } = openAdd()

    await user.click(await screen.findByLabelText('Entries'))
    await user.paste('coffee 60')
    await user.click(screen.getByRole('button', { name: 'Read entries' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong on the server')
    expect(screen.getByLabelText('Entries')).toHaveValue('coffee 60')
  })

  it('lists every proposal as a row and marks the unsure one', async () => {
    const { sheet } = await readText()

    expect(within(sheet).getByText('2 entries. Tap one to change it.')).toBeInTheDocument()
    const rows = within(sheet).getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    const [first, second] = rows as [HTMLElement, HTMLElement]
    expect(first).toHaveTextContent('coffee')
    expect(first).toHaveTextContent('Food · Sun 4 Oct')
    expect(first).toHaveTextContent('60.00')
    expect(within(first).queryByText('Check this')).toBeNull()
    expect(second).toHaveTextContent('Transport · Sat 3 Oct')
    expect(second).toHaveTextContent('145.00')
    expect(within(second).getByText('Check this')).toBeInTheDocument()
    // No AI notice when the AI was used; the keyboard stays down (focus is not in a field).
    expect(within(sheet).queryByText(/AI/)).toBeNull()
    expect(document.activeElement?.tagName).not.toBe('INPUT')
  })

  it('shows income in green with a plus, and treats an archived category as none', async () => {
    parsed([
      {
        ...coffee,
        text: 'salary 30000',
        merchant: 'salary',
        amount: '30000.00',
        category_id: salary.id,
      },
      { ...coffee, text: 'box 10', merchant: 'box', amount: '10.00', category_id: old.id },
    ])
    const { sheet } = await readText('salary 30000, box 10')

    const amount = within(row(sheet, 'salary')).getByText('+30,000')
    expect(amount.closest('[data-slot="amount"]')).toHaveClass('text-income')
    expect(row(sheet, 'box')).toHaveTextContent('Choose a category.')
  })

  it.each([
    { ai: 'limit_reached', notice: "Today's AI limit is reached. Choose the categories yourself." },
    { ai: 'unavailable', notice: 'The AI could not be reached. Choose the categories yourself.' },
    { ai: 'not_configured', notice: 'The AI is not set up. Choose the categories yourself.' },
  ])('says so when the AI was $ai', async ({ ai, notice }) => {
    parsed([gift], ai)
    const { sheet } = await readText('gift 500')

    expect(within(sheet).getByText(notice)).toBeInTheDocument()
    expect(row(sheet, 'gift')).toHaveTextContent('Choose a category.')
  })

  it('saves edited rows with source text, the typed item and their own ids', async () => {
    const { user, sheet } = await readText()

    await user.click(row(sheet, 'coffee'))
    const description = within(sheet).getByLabelText('Description')
    await user.clear(description)
    await user.type(description, 'Latte')
    const amount = within(sheet).getByLabelText('Amount')
    await user.clear(amount)
    await user.type(amount, '65')
    await user.type(within(sheet).getByLabelText('Note'), 'oat milk')
    // Only one row is open at a time.
    await user.click(row(sheet, 'grab'))
    expect(within(sheet).getAllByLabelText('Amount')).toHaveLength(1)
    expect(within(sheet).getByLabelText('Description')).toHaveValue('grab')
    // Changing the unsure row takes its tag away.
    await user.click(within(sheet).getByRole('radio', { name: 'Food' }))
    expect(within(sheet).queryByText('Check this')).toBeNull()

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    expect(await screen.findByText('Saved 2 transactions.')).toBeInTheDocument()
    const [first, second] = posted() as [Record<string, string>, Record<string, string>]
    expect(first).toEqual({
      id: expect.stringMatching(v7) as string,
      amount: '65.00',
      occurred_on: '2026-10-04',
      category_id: food.id,
      merchant: 'Latte',
      note: 'oat milk',
      source: 'text',
      raw_input: 'coffee 60',
    })
    expect(second).toEqual({
      id: expect.stringMatching(v7) as string,
      amount: '145.00',
      occurred_on: '2026-10-03',
      category_id: food.id,
      merchant: 'grab',
      source: 'text',
      raw_input: 'grab 145 yesterday',
    })
    expect(first.id).not.toBe(second.id)
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(screen.getByLabelText('Entries')).toHaveValue('')
  })

  it('removes a row; removing the last closes the sheet and keeps the text', async () => {
    const { user, sheet } = await readText()

    await user.click(within(sheet).getByRole('button', { name: 'Remove coffee' }))
    expect(within(sheet).getAllByRole('listitem')).toHaveLength(1)
    expect(within(sheet).getByText('1 entry. Tap it to change it.')).toBeInTheDocument()

    await user.click(within(sheet).getByRole('button', { name: 'Remove grab' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(screen.getByLabelText('Entries')).toHaveValue('coffee 60, grab 145 yesterday')
    expect(posted()).toHaveLength(0)
  })

  it('saves only what is left after a remove', async () => {
    const { user, sheet } = await readText()

    await user.click(within(sheet).getByRole('button', { name: 'Remove grab' }))
    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    expect(await screen.findByText('Saved 1 transaction.')).toBeInTheDocument()
    expect(posted().map((b) => b.raw_input)).toEqual(['coffee 60'])
  })

  it('offers to save the ready ones first, then finishes the rest', async () => {
    parsed([coffee, gift], 'limit_reached')
    const { user, sheet } = await readText('coffee 60, gift 500')

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    expect(within(sheet).getByText('1 entry is not finished yet.')).toBeInTheDocument()
    expect(posted()).toHaveLength(0)
    await user.click(within(sheet).getByRole('button', { name: 'Save the ready one' }))

    await waitFor(() => {
      expect(posted().map((b) => b.raw_input)).toEqual(['coffee 60'])
    })
    await waitFor(() => {
      expect(within(sheet).getAllByRole('listitem')).toHaveLength(1)
    })
    // The rest stays, opened with its error.
    expect(within(sheet).getByRole('group', { name: 'Category' })).toHaveAccessibleDescription(
      'Choose a category.',
    )
    await user.click(within(sheet).getByRole('radio', { name: 'Transport' }))
    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    expect(await screen.findByText('Saved 2 transactions.')).toBeInTheDocument()
    expect(posted()[1]).toMatchObject({ raw_input: 'gift 500', category_id: transport.id })
    expect(screen.getByLabelText('Entries')).toHaveValue('')
  })

  it('lets the user finish the rest first, saving nothing', async () => {
    parsed([coffee, gift], 'limit_reached')
    const { user, sheet } = await readText('coffee 60, gift 500')

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))
    await user.click(within(sheet).getByRole('button', { name: 'Finish the rest first' }))

    expect(within(sheet).getByRole('group', { name: 'Category' })).toHaveAccessibleDescription(
      'Choose a category.',
    )
    expect(row(sheet, 'gift')).toHaveAttribute('aria-expanded', 'true')
    expect(within(sheet).getAllByRole('listitem')).toHaveLength(2)
    expect(within(sheet).getByRole('button', { name: 'Save all' })).toBeInTheDocument()
    expect(posted()).toHaveLength(0)
  })

  it('opens the first incomplete one when none is ready', async () => {
    parsed([gift, { ...gift, text: 'x', merchant: 'x', amount: '' }], 'unavailable')
    const { user, sheet } = await readText('gift 500, x')

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    expect(within(sheet).queryByText(/not finished/)).toBeNull()
    expect(row(sheet, 'gift')).toHaveAttribute('aria-expanded', 'true')
    expect(row(sheet, 'x')).toHaveTextContent('Enter an amount. Choose a category.')
    expect(posted()).toHaveLength(0)
  })

  it('keeps a failed row with its error and retries it with the same id', async () => {
    let attempts = 0
    api.on('POST /api/transactions', (call) => {
      if ((call.body as { raw_input: string }).raw_input === 'grab 145 yesterday') {
        attempts++
        if (attempts === 1) {
          return Promise.reject(new TypeError('offline'))
        }
      }
      return savedBody(call)
    })
    const { user, sheet } = await readText()

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    await waitFor(() => {
      expect(within(sheet).getAllByRole('listitem')).toHaveLength(1)
    })
    expect(within(sheet).getByRole('alert')).toHaveTextContent('The server could not be reached')
    expect(row(sheet, 'grab')).toBeInTheDocument()

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    expect(await screen.findByText('Saved 2 transactions.')).toBeInTheDocument()
    const grabs = posted().filter((b) => b.raw_input === 'grab 145 yesterday')
    expect(grabs).toHaveLength(2)
    expect(grabs[1]?.id).toBe(grabs[0]?.id)
    expect(posted()).toHaveLength(3)
  })

  it('opens a row the API refused, with the error beside its field', async () => {
    api.on('POST /api/transactions', (call) =>
      (call.body as { raw_input: string }).raw_input === 'coffee 60'
        ? apiError(
            400,
            'invalid_input',
            'choose one of your active categories (create or unarchive one if there is none)',
          )
        : savedBody(call),
    )
    const { user, sheet } = await readText()

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))

    await waitFor(() => {
      expect(within(sheet).getByRole('group', { name: 'Category' })).toHaveAccessibleDescription(
        'Choose one of your active categories.',
      )
    })
    expect(row(sheet, 'coffee')).toHaveAttribute('aria-expanded', 'true')
    expect(within(sheet).getAllByRole('listitem')).toHaveLength(1)
    // The categories are loaded again in case one was archived elsewhere.
    expect(api.callsTo('GET /api/categories').length).toBeGreaterThan(1)
  })

  it('keeps the text when the sheet is closed without saving', async () => {
    const { user, sheet } = await readText()

    await user.click(within(sheet).getByRole('button', { name: 'Cancel' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(screen.getByLabelText('Entries')).toHaveValue('coffee 60, grab 145 yesterday')
    expect(screen.queryByText(/Saved/)).toBeNull()
    expect(posted()).toHaveLength(0)
  })

  it('says how many were saved and keeps only the rest when closed after a partial save', async () => {
    parsed([coffee, gift], 'limit_reached')
    const { user, sheet } = await readText('coffee 60, gift 500')

    await user.click(within(sheet).getByRole('button', { name: 'Save all' }))
    await user.click(within(sheet).getByRole('button', { name: 'Save the ready one' }))
    await waitFor(() => {
      expect(within(sheet).getAllByRole('listitem')).toHaveLength(1)
    })
    await user.click(within(sheet).getByRole('button', { name: 'Cancel' }))

    expect(
      await screen.findByText('Saved 1 transaction. The rest were not saved.'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Entries')).toHaveValue('gift 500')
  })
})
