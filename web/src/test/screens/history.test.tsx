import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { RecordedCall } from '@/test/api'
import { apiError, category, json, mockApi, noContent, profile, transaction } from '@/test/api'
import { renderApp } from '@/test/render'

const food = category(1, { name: 'Food', icon: '🍜' })
const transport = category(2, { name: 'Transport', icon: '🚌' })
const salary = category(3, { name: 'Salary', icon: '💼', kind: 'income' })
const oldIncome = category(4, { name: 'Old job', icon: '🏢', kind: 'income', archived: true })

// Newest first, as the API sends them.
const lunch = transaction(1, {
  occurred_on: '2026-10-04',
  amount: '145.00',
  merchant: 'Noodle shop',
  category_id: food.id,
  note: 'with Kan',
})
const pay = transaction(2, {
  occurred_on: '2026-10-04',
  amount: '30000.00',
  category_id: salary.id,
})
const bus = transaction(3, { occurred_on: '2026-10-03', amount: '0.10', category_id: transport.id })
const bus2 = transaction(4, {
  occurred_on: '2026-10-03',
  amount: '0.20',
  category_id: transport.id,
})
const bonus = transaction(5, {
  occurred_on: '2026-10-01',
  amount: '500.00',
  merchant: 'Last payment',
  category_id: oldIncome.id,
})

let api: ReturnType<typeof mockApi>
let rows: unknown[]

beforeEach(() => {
  // Noon on Sunday 4 October 2026 in Bangkok.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-04T05:00:00Z'))
  rows = [lunch, pay, bus, bus2, bonus]
  api = mockApi({
    'GET /api/me': () => json(200, profile),
    'GET /api/categories': () => json(200, { categories: [food, transport, salary, oldIncome] }),
    'GET /api/transactions': () => json(200, { transactions: rows, next_cursor: null }),
  })
})

afterEach(() => {
  vi.useRealTimers()
})

function openHistory() {
  return renderApp('/history', { token: 'tok' })
}

function listQueries(): string[] {
  return api.callsTo('GET /api/transactions').map((c: RecordedCall) => c.query.toString())
}

async function day(label: string) {
  return screen.findByRole('region', { name: label })
}

describe('History', () => {
  it('opens on this month, from /', async () => {
    const { router } = renderApp('/', { token: 'tok' })

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/history')
    })
    expect(await screen.findByRole('heading', { name: 'October 2026' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Month' })).toBeChecked()
    await day('Sun 4 Oct')
    expect(listQueries()).toEqual(['month=2026-10&limit=200'])
  })

  it('shows placeholders while loading', async () => {
    openHistory()

    expect(await screen.findByTestId('history-skeleton')).toBeInTheDocument()
    await day('Sun 4 Oct')
    expect(screen.queryByTestId('history-skeleton')).toBeNull()
  })

  it('groups rows by day, newest first, with expense and income totals', async () => {
    openHistory()

    const regions = await screen.findAllByRole('region')
    expect(regions.map((r) => r.querySelector('h3')?.textContent)).toEqual([
      'Sun 4 Oct',
      'Sat 3 Oct',
      'Thu 1 Oct',
    ])

    const sunday = await day('Sun 4 Oct')
    expect(within(sunday).getByTestId('day-expenses')).toHaveTextContent('Spent 145.00')
    expect(within(sunday).getByTestId('day-income')).toHaveTextContent('Received +30,000.00')
    const rowButtons = within(sunday).getAllByRole('button')
    expect(rowButtons[0]).toHaveTextContent('Noodle shop')
    expect(rowButtons[0]).toHaveTextContent('Food')
    expect(rowButtons[0]).toHaveTextContent('145.00')
    // No merchant: the category name is the row's title.
    expect(rowButtons[1]).toHaveTextContent('Salary')
    expect(rowButtons[1]).toHaveTextContent('+30,000.00')

    // 0.10 + 0.20 is exactly 0.30, not a float's 0.30000000000000004; no income, no income total.
    const saturday = await day('Sat 3 Oct')
    expect(within(saturday).getByTestId('day-expenses')).toHaveTextContent('Spent 0.30')
    expect(within(saturday).queryByTestId('day-income')).toBeNull()

    // An archived category still names its rows and makes them income.
    const thursday = await day('Thu 1 Oct')
    expect(within(thursday).getByRole('button')).toHaveTextContent('Old job')
    expect(within(thursday).getByTestId('day-expenses')).toHaveTextContent('Spent 0.00')
    expect(within(thursday).getByTestId('day-income')).toHaveTextContent('+500.00')
  })

  it('follows next_cursor until every page of the period is in', async () => {
    const pages: Record<string, { transactions: unknown[]; next_cursor: string | null }> = {
      '': { transactions: [lunch, pay], next_cursor: 'p2' },
      p2: { transactions: [bus], next_cursor: 'p3' },
      p3: { transactions: [bus2], next_cursor: null },
    }
    api.on('GET /api/transactions', (call) =>
      json(200, pages[call.query.get('cursor') ?? ''] ?? {}),
    )
    openHistory()

    const saturday = await day('Sat 3 Oct')
    expect(within(saturday).getAllByRole('button')).toHaveLength(2)
    expect(within(saturday).getByTestId('day-expenses')).toHaveTextContent('0.30')
    expect(listQueries()).toEqual([
      'month=2026-10&limit=200',
      'month=2026-10&limit=200&cursor=p2',
      'month=2026-10&limit=200&cursor=p3',
    ])
  })

  it('switches to the week, Monday to Sunday, and remembers it', async () => {
    const { user } = openHistory()

    await day('Sun 4 Oct')
    await user.click(screen.getByRole('radio', { name: 'Week' }))

    expect(await screen.findByRole('heading', { name: '28 Sept – 4 Oct 2026' })).toBeInTheDocument()
    await waitFor(() => {
      expect(listQueries().at(-1)).toBe('from=2026-09-28&to=2026-10-04&limit=200')
    })
    expect(localStorage.getItem('satang.history-view')).toBe('week')

    await user.click(screen.getByRole('button', { name: 'Previous week' }))
    expect(await screen.findByRole('heading', { name: '21 – 27 Sept 2026' })).toBeInTheDocument()
    await waitFor(() => {
      expect(listQueries().at(-1)).toBe('from=2026-09-21&to=2026-09-27&limit=200')
    })
  })

  it('opens on the remembered week', async () => {
    localStorage.setItem('satang.history-view', 'week')
    openHistory()

    expect(await screen.findByRole('heading', { name: '28 Sept – 4 Oct 2026' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Week' })).toBeChecked()
    await day('Sun 4 Oct')
    expect(listQueries()).toEqual(['from=2026-09-28&to=2026-10-04&limit=200'])
  })

  it('moves between months', async () => {
    const { user } = openHistory()

    await day('Sun 4 Oct')
    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    expect(await screen.findByRole('heading', { name: 'September 2026' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Next month' }))
    await user.click(screen.getByRole('button', { name: 'Next month' }))
    expect(await screen.findByRole('heading', { name: 'November 2026' })).toBeInTheDocument()
    await waitFor(() => {
      expect(listQueries()).toEqual([
        'month=2026-10&limit=200',
        'month=2026-09&limit=200',
        // Back on October, its stale list is loaded again.
        'month=2026-10&limit=200',
        'month=2026-11&limit=200',
      ])
    })
  })

  it('says so honestly when the period is empty', async () => {
    rows = []
    openHistory()

    expect(await screen.findByText('Nothing recorded for October 2026.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Add an expense or income' })).toHaveAttribute(
      'href',
      '/add',
    )
  })

  it('shows an error with Try again', async () => {
    let fail = true
    api.on('GET /api/transactions', () =>
      fail
        ? apiError(500, 'internal', 'internal error')
        : json(200, { transactions: [lunch], next_cursor: null }),
    )
    const { user } = openHistory()

    expect(await screen.findByRole('alert')).toHaveTextContent('Your history could not be loaded.')
    fail = false
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await day('Sun 4 Oct')).toHaveTextContent('Noodle shop')
    expect(screen.queryByRole('alert')).toBeNull()
  })
})

describe('Transaction detail', () => {
  async function openRow(name: RegExp) {
    const view = openHistory()
    await view.user.click(await screen.findByRole('button', { name }))
    return view
  }

  it('shows the row and saves only the changed fields', async () => {
    api.on(`PATCH /api/transactions/${lunch.id}`, () => json(200, { ...lunch, amount: '150.00' }))
    const { user } = await openRow(/Noodle shop/)

    const sheet = await screen.findByRole('dialog', { name: 'Noodle shop' })
    expect(sheet).toHaveTextContent('Sun 4 Oct 2026 · Food')
    expect(within(sheet).getByLabelText('Amount')).toHaveValue('145.00')
    expect(within(sheet).getByRole('radio', { name: 'Food' })).toBeChecked()
    expect(within(sheet).getByLabelText('Date')).toHaveValue('2026-10-04')
    expect(within(sheet).getByLabelText('Merchant')).toHaveValue('Noodle shop')
    expect(within(sheet).getByLabelText('Note')).toHaveValue('with Kan')

    const amount = within(sheet).getByLabelText('Amount')
    await user.clear(amount)
    await user.type(amount, '150')
    await user.click(within(sheet).getByRole('radio', { name: 'Transport' }))
    await user.click(within(sheet).getByRole('button', { name: 'Save changes' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Changes saved.')
    expect(api.onlyCall(`PATCH /api/transactions/${lunch.id}`).body).toEqual({
      amount: '150.00',
      category_id: transport.id,
    })
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    // The list is loaded again.
    await waitFor(() => {
      expect(listQueries()).toHaveLength(2)
    })
  })

  it('keeps an archived category and sends a cleared merchant as empty', async () => {
    api.on(`PATCH /api/transactions/${bonus.id}`, () => json(200, bonus))
    const { user } = await openRow(/Last payment/)

    const sheet = await screen.findByRole('dialog', { name: 'Last payment' })
    expect(within(sheet).getByRole('radio', { name: 'Income' })).toBeChecked()
    expect(within(sheet).getByRole('radio', { name: 'Old job (Archived)' })).toBeChecked()
    await user.clear(within(sheet).getByLabelText('Merchant'))
    await user.click(within(sheet).getByRole('button', { name: 'Save changes' }))

    await waitFor(() => {
      expect(api.callsTo(`PATCH /api/transactions/${bonus.id}`)).toHaveLength(1)
    })
    expect(api.onlyCall(`PATCH /api/transactions/${bonus.id}`).body).toEqual({ merchant: '' })
  })

  it('sends nothing when nothing changed', async () => {
    const { user } = await openRow(/Noodle shop/)

    const sheet = await screen.findByRole('dialog', { name: 'Noodle shop' })
    await user.click(within(sheet).getByRole('button', { name: 'Save changes' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(api.calls.filter((c) => c.method === 'PATCH')).toHaveLength(0)
  })

  it('checks the fields before sending', async () => {
    const { user } = await openRow(/Noodle shop/)

    const sheet = await screen.findByRole('dialog', { name: 'Noodle shop' })
    await user.clear(within(sheet).getByLabelText('Amount'))
    await user.click(within(sheet).getByRole('button', { name: 'Save changes' }))

    expect(within(sheet).getByLabelText('Amount')).toHaveAccessibleDescription('Enter an amount.')
    expect(api.calls.filter((c) => c.method === 'PATCH')).toHaveLength(0)
  })

  it('deletes after a confirmation, and refreshes the list', async () => {
    api.on(`DELETE /api/transactions/${lunch.id}`, () => {
      rows = [pay, bus, bus2, bonus]
      return noContent()
    })
    const { user } = await openRow(/Noodle shop/)

    let sheet = await screen.findByRole('dialog', { name: 'Noodle shop' })
    await user.click(within(sheet).getByRole('button', { name: 'Delete' }))
    sheet = await screen.findByRole('dialog', { name: 'Delete this transaction?' })
    expect(sheet).toHaveTextContent('The 145.00 on Sun 4 Oct 2026 is removed for good.')
    expect(api.callsTo(`DELETE /api/transactions/${lunch.id}`)).toHaveLength(0)
    await user.click(within(sheet).getByRole('button', { name: 'Delete' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Transaction deleted.')
    expect(api.callsTo(`DELETE /api/transactions/${lunch.id}`)).toHaveLength(1)
    await waitFor(() => {
      expect(screen.queryByText('Noodle shop')).toBeNull()
    })
  })

  it('goes back from the confirmation without deleting', async () => {
    const { user } = await openRow(/Noodle shop/)

    await user.click(
      within(await screen.findByRole('dialog', { name: 'Noodle shop' })).getByRole('button', {
        name: 'Delete',
      }),
    )
    const sheet = await screen.findByRole('dialog', { name: 'Delete this transaction?' })
    await user.click(within(sheet).getByRole('button', { name: 'Cancel' }))

    expect(await screen.findByRole('dialog', { name: 'Noodle shop' })).toBeInTheDocument()
    expect(api.calls.filter((c) => c.method === 'DELETE')).toHaveLength(0)
  })
})
