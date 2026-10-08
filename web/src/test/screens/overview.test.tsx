import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { invalidateCategories, invalidateTransactions } from '@/queries/money'
import { apiError, json, mockApi, profile } from '@/test/api'
import { renderApp } from '@/test/render'

/** A dashboard body as the API sends it (ADR-0089); `month` defaults to October 2026. */
function dashboard(
  fields: Partial<{
    month: string
    totals: { spent: string; income: string; difference: string }
    categories: unknown[]
  }> = {},
) {
  const month = fields.month ?? '2026-10'
  return {
    month,
    totals: { spent: '0.00', income: '0.00', difference: '0.00' },
    categories: [],
    trend: [
      { month: '2026-05', spent: '0.00' },
      { month: '2026-06', spent: '0.00' },
      { month: '2026-07', spent: '0.00' },
      { month: '2026-08', spent: '0.00' },
      { month: '2026-09', spent: '0.00' },
      { month, spent: '0.00' },
    ],
    top_descriptions: [],
    ...fields,
  }
}

function row(n: number, name: string, icon: string, total: string, archived = false) {
  return {
    category_id: `0190a1b2-0000-7000-8000-1000000000${String(n).padStart(2, '0')}`,
    name,
    icon,
    archived,
    total,
    count: 1,
  }
}

const october = dashboard({
  totals: { spent: '4000.00', income: '30000.00', difference: '26000.00' },
  categories: [
    row(1, 'Food', '🍜', '3000.00'),
    row(2, 'Transport', '🚌', '985.00'),
    row(3, 'Old hobby', '', '15.00', true),
  ],
})

let api: ReturnType<typeof mockApi>
let body: unknown

beforeEach(() => {
  // Noon on Sunday 4 October 2026 in Bangkok.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-04T05:00:00Z'))
  body = october
  api = mockApi({
    'GET /api/me': () => json(200, profile),
    'GET /api/dashboard': () => json(200, body),
  })
})

afterEach(() => {
  vi.useRealTimers()
})

function openOverview() {
  return renderApp('/overview', { token: 'tok' })
}

function months(): (string | null)[] {
  return api.callsTo('GET /api/dashboard').map((c) => c.query.get('month'))
}

async function breakdown() {
  return screen.findByRole('region', { name: 'Spending by category' })
}

describe('Overview', () => {
  it('is the second place in the bar and opens on this month', async () => {
    openOverview()

    expect(await screen.findByRole('heading', { name: 'October 2026' })).toBeInTheDocument()
    const bar = screen.getByRole('navigation', { name: 'Main' })
    const links = within(bar).getAllByRole('link')
    expect(links.map((l) => l.textContent)).toEqual(['History', 'Overview', 'Add', 'Settings'])
    expect(within(bar).getByRole('link', { name: 'Overview' })).toHaveAttribute(
      'data-status',
      'active',
    )
    await breakdown()
    expect(months()).toEqual(['2026-10'])
  })

  it('shows placeholders while loading', async () => {
    openOverview()

    expect(await screen.findByTestId('overview-skeleton')).toBeInTheDocument()
    await breakdown()
    expect(screen.queryByTestId('overview-skeleton')).toBeNull()
  })

  it('shows the totals: income and a positive difference in green with a plus', async () => {
    openOverview()

    await breakdown()
    expect(screen.getByTestId('total-spent')).toHaveTextContent('Spent4,000.00')
    expect(screen.getByTestId('total-income')).toHaveTextContent('Income+30,000.00')
    expect(screen.getByTestId('total-difference')).toHaveTextContent('Difference+26,000.00')
    expect(
      within(screen.getByTestId('total-difference')).getByText('+26,000', { exact: false })
        .parentElement,
    ).toHaveClass('text-income')
  })

  it('shows a negative difference in ink with a minus', async () => {
    body = dashboard({
      totals: { spent: '500.25', income: '100.00', difference: '-400.25' },
      categories: [row(1, 'Food', '🍜', '500.25')],
    })
    openOverview()

    await breakdown()
    const difference = screen.getByTestId('total-difference')
    expect(difference).toHaveTextContent('Difference-400.25')
    expect(difference.querySelector('[data-slot="amount"]')).toHaveClass('text-foreground')
  })

  it('lists spending by category, largest first, with share and a bar against the largest', async () => {
    openOverview()

    const region = await breakdown()
    const rows = within(region).getAllByTestId('category-row')
    expect(rows).toHaveLength(3)

    expect(rows[0]).toHaveTextContent('Food')
    expect(rows[0]).toHaveTextContent('3,000.00')
    expect(rows[0]).toHaveTextContent('75% of spending')
    // 985 / 4000 is 24.625%: rounded half up to 25%.
    expect(rows[1]).toHaveTextContent('Transport')
    expect(rows[1]).toHaveTextContent('25% of spending')
    // A share above zero that rounds to 0 is "<1%"; an archived category says so.
    expect(rows[2]).toHaveTextContent('Old hobby')
    expect(rows[2]).toHaveTextContent('Archived')
    expect(rows[2]).toHaveTextContent('<1% of spending')

    const bars = within(region).getAllByTestId('category-bar')
    expect(bars.map((b) => b.style.width)).toEqual(['100%', '32.83%', '0.5%'])
  })

  it('shows the totals and "No spending" for a month with only income', async () => {
    body = dashboard({ totals: { spent: '0.00', income: '30000.00', difference: '30000.00' } })
    openOverview()

    const region = await breakdown()
    expect(within(region).getByText('No spending this month.')).toBeInTheDocument()
    expect(screen.getByTestId('total-spent')).toHaveTextContent('0.00')
    expect(screen.getByTestId('total-income')).toHaveTextContent('+30,000.00')
  })

  it('has an empty state with a way to Add for a month without transactions', async () => {
    body = dashboard()
    openOverview()

    expect(await screen.findByText('Nothing recorded for October 2026.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Add an expense or income' })).toHaveAttribute(
      'href',
      '/add',
    )
    expect(screen.queryByRole('region', { name: 'Spending by category' })).toBeNull()
  })

  it('moves between months with the arrows', async () => {
    const { user } = openOverview()
    await breakdown()

    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    expect(await screen.findByRole('heading', { name: 'September 2026' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    expect(await screen.findByRole('heading', { name: 'July 2026' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Next month' }))
    expect(await screen.findByRole('heading', { name: 'August 2026' })).toBeInTheDocument()
    // August was seen before: it shows at once and is checked again in the background.
    await waitFor(() => {
      expect(months()).toEqual(['2026-10', '2026-09', '2026-08', '2026-07', '2026-08'])
    })
  })

  it('stops at one year after this month', async () => {
    const { user } = openOverview()
    await breakdown()

    const next = screen.getByRole('button', { name: 'Next month' })
    for (let i = 0; i < 12; i++) {
      await user.click(next)
    }
    expect(await screen.findByRole('heading', { name: 'October 2027' })).toBeInTheDocument()
    expect(next).toHaveAttribute('aria-disabled', 'true')
    await user.click(next)
    expect(screen.getByRole('heading', { name: 'October 2027' })).toBeInTheDocument()
    expect(months()).not.toContain('2027-11')
  })

  it('stops at January 2000', async () => {
    vi.setSystemTime(new Date('2000-02-15T05:00:00Z'))
    body = dashboard({ month: '2000-02' })
    const { user } = openOverview()
    expect(await screen.findByRole('heading', { name: 'February 2000' })).toBeInTheDocument()

    const previous = screen.getByRole('button', { name: 'Previous month' })
    expect(previous).not.toHaveAttribute('aria-disabled', 'true')
    await user.click(previous)
    expect(await screen.findByRole('heading', { name: 'January 2000' })).toBeInTheDocument()
    expect(previous).toHaveAttribute('aria-disabled', 'true')
    await user.click(previous)
    expect(screen.getByRole('heading', { name: 'January 2000' })).toBeInTheDocument()
    await waitFor(() => {
      expect(months()).toEqual(['2000-02', '2000-01'])
    })
  })

  it('shows an error with Try again', async () => {
    api.on('GET /api/dashboard', () => apiError(500, 'internal', 'boom'))
    const { user } = openOverview()

    expect(await screen.findByText('Your overview could not be loaded.')).toBeInTheDocument()
    api.on('GET /api/dashboard', () => json(200, october))
    await user.click(screen.getByRole('button', { name: 'Try again' }))
    await breakdown()
    expect(screen.queryByText('Your overview could not be loaded.')).toBeNull()
  })

  it('is loaded again after a transaction or a category changes', async () => {
    const { queryClient } = openOverview()
    await breakdown()
    expect(months()).toEqual(['2026-10'])

    await invalidateTransactions(queryClient)
    await waitFor(() => {
      expect(months()).toEqual(['2026-10', '2026-10'])
    })
    await invalidateCategories(queryClient)
    await waitFor(() => {
      expect(months()).toEqual(['2026-10', '2026-10', '2026-10'])
    })
  })
})
