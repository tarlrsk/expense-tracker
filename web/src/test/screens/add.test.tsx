import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { apiError, category, json, mockApi, profile, transaction } from '@/test/api'
import { renderApp } from '@/test/render'

const food = category(1, { name: 'Food', icon: '🍜' })
const transport = category(2, { name: 'Transport', icon: '🚌' })
const salary = category(3, { name: 'Salary', icon: '💼', kind: 'income' })
const old = category(4, { name: 'Old', icon: '📦', archived: true })

const v7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  // Noon on Sunday 4 October 2026 in Bangkok.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-04T05:00:00Z'))
  api = mockApi({
    'GET /api/me': () => json(200, profile),
    'GET /api/categories': () => json(200, { categories: [food, transport, salary, old] }),
    'POST /api/transactions': (call) => json(201, { ...transaction(1), ...(call.body as object) }),
    'GET /api/transactions': () => json(200, { transactions: [], next_cursor: null }),
  })
})

afterEach(() => {
  vi.useRealTimers()
})

// These are the manual form's tests: Add opens on Quick unless Form was used last.
function openAdd() {
  localStorage.setItem('satang.add-mode', 'form')
  return renderApp('/add', { token: 'tok' })
}

describe('Add, Form mode', () => {
  it('asks for the amount first, with the number pad, and today as the date', async () => {
    openAdd()

    const amount = await screen.findByLabelText('Amount')
    expect(amount).toHaveAttribute('inputmode', 'decimal')
    expect(screen.getByRole('radio', { name: 'Expense' })).toBeChecked()
    const date = screen.getByLabelText('Date')
    expect(date).toHaveValue('2026-10-04')
    expect(date).toHaveAttribute('min', '2000-01-01')
    expect(date).toHaveAttribute('max', '2027-10-04')
    // Placeholders while the categories load, then the active expense ones in the user's order.
    expect(screen.getByTestId('categories-skeleton')).toBeInTheDocument()
    const grid = await screen.findByRole('group', { name: 'Category' })
    expect(
      within(grid)
        .getAllByRole('radio')
        .map((r) => r.closest('label')?.textContent),
    ).toEqual(['🍜Food', '🚌Transport'])
  })

  it('shows the income categories after switching to Income', async () => {
    const { user } = openAdd()

    await user.click(await screen.findByRole('radio', { name: 'Food' }))
    await user.click(screen.getByRole('radio', { name: 'Income' }))

    const grid = screen.getByRole('group', { name: 'Category' })
    expect(within(grid).getAllByRole('radio')).toHaveLength(1)
    expect(within(grid).getByRole('radio', { name: 'Salary' })).not.toBeChecked()
  })

  it.each([
    { amount: '', error: 'Enter an amount.' },
    { amount: '0', error: 'Enter an amount more than 0.' },
    { amount: '12.345', error: 'Use at most two decimals.' },
    { amount: '1,000', error: 'Use digits and a point, such as 145.50.' },
    { amount: '10000000000', error: 'Use at most 9,999,999,999.99.' },
  ])('checks the amount $amount before sending', async ({ amount, error }) => {
    const { user } = openAdd()

    const field = await screen.findByLabelText('Amount')
    if (amount !== '') {
      await user.type(field, amount)
    }
    await user.click(await screen.findByRole('radio', { name: 'Food' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(field).toHaveAccessibleDescription(error)
    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(api.callsTo('POST /api/transactions')).toHaveLength(0)
  })

  it('asks for a category and checks the merchant beside their fields', async () => {
    const { user } = openAdd()

    await user.type(await screen.findByLabelText('Amount'), '145')
    await user.type(screen.getByLabelText('Description'), 'm'.repeat(101))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(screen.getByRole('group', { name: 'Category' })).toHaveAccessibleDescription(
      'Choose a category.',
    )
    expect(screen.getByLabelText('Description')).toHaveAccessibleDescription(
      expect.stringContaining('Use at most 100 characters.'),
    )
    expect(api.callsTo('POST /api/transactions')).toHaveLength(0)
  })

  it('saves with a new UUID v7, says so, and clears the form for the next one', async () => {
    const { user } = openAdd()

    await user.type(await screen.findByLabelText('Amount'), '145')
    await user.click(await screen.findByRole('radio', { name: 'Food' }))
    await user.type(screen.getByLabelText('Description'), '  Noodle shop ')
    await user.type(screen.getByLabelText('Note'), 'with Kan')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Saved.')
    const first = api.onlyCall('POST /api/transactions').body as { id: string }
    expect(first).toEqual({
      id: expect.stringMatching(v7) as string,
      amount: '145.00',
      occurred_on: '2026-10-04',
      category_id: food.id,
      merchant: 'Noodle shop',
      note: 'with Kan',
    })
    expect(screen.getByLabelText('Amount')).toHaveValue('')
    expect(screen.getByLabelText('Description')).toHaveValue('')
    expect(screen.getByLabelText('Note')).toHaveValue('')
    expect(screen.getByRole('radio', { name: 'Food' })).not.toBeChecked()
    expect(screen.getByRole('radio', { name: 'Expense' })).toBeChecked()

    // The next entry gets its own id; merchant and note are left out when empty.
    await user.type(screen.getByLabelText('Amount'), '20.5')
    await user.click(screen.getByRole('radio', { name: 'Income' }))
    await user.click(screen.getByRole('radio', { name: 'Salary' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(api.callsTo('POST /api/transactions')).toHaveLength(2)
    })
    const second = api.callsTo('POST /api/transactions')[1]?.body as { id: string }
    expect(second).toEqual({
      id: expect.stringMatching(v7) as string,
      amount: '20.50',
      occurred_on: '2026-10-04',
      category_id: salary.id,
    })
    expect(second.id).not.toBe(first.id)
  })

  it('reuses the same id when a failed send is retried, so it saves once', async () => {
    let attempts = 0
    api.on('POST /api/transactions', (call) => {
      attempts++
      return attempts === 1
        ? Promise.reject(new TypeError('offline'))
        : json(201, { ...transaction(1), ...(call.body as object) })
    })
    const { user } = openAdd()

    await user.type(await screen.findByLabelText('Amount'), '145')
    await user.click(await screen.findByRole('radio', { name: 'Food' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('The server could not be reached')
    expect(screen.getByLabelText('Amount')).toHaveValue('145')

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Saved.')
    const ids = api.callsTo('POST /api/transactions').map((c) => (c.body as { id: string }).id)
    expect(ids).toHaveLength(2)
    expect(ids[0]).toMatch(v7)
    expect(ids[1]).toBe(ids[0])
  })

  it('shows an API refusal of the category beside the categories', async () => {
    api.on('POST /api/transactions', () =>
      apiError(
        400,
        'invalid_input',
        'choose one of your active categories (create or unarchive one if there is none)',
      ),
    )
    const { user } = openAdd()

    await user.type(await screen.findByLabelText('Amount'), '145')
    await user.click(await screen.findByRole('radio', { name: 'Food' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(screen.getByRole('group', { name: 'Category' })).toHaveAccessibleDescription(
        'Choose one of your active categories.',
      )
    })
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('says plainly when there is no active category of the chosen kind', async () => {
    api.on('GET /api/categories', () => json(200, { categories: [salary, old] }))
    const { user } = openAdd()

    expect(
      await screen.findByText('You have no expense categories to choose from.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Add one in Categories' })).toHaveAttribute(
      'href',
      '/settings/categories',
    )

    await user.click(screen.getByRole('radio', { name: 'Income' }))
    expect(screen.getByRole('radio', { name: 'Salary' })).toBeInTheDocument()
    expect(screen.queryByText(/no income categories/)).toBeNull()
  })

  it('offers Try again when the categories cannot be loaded', async () => {
    let fail = true
    api.on('GET /api/categories', () =>
      fail ? apiError(500, 'internal', 'internal error') : json(200, { categories: [food] }),
    )
    const { user } = openAdd()

    expect(await screen.findByText('Your categories could not be loaded.')).toBeInTheDocument()
    fail = false
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByRole('radio', { name: 'Food' })).toBeInTheDocument()
  })
})
