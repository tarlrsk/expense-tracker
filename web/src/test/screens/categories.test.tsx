import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { apiError, category, json, mockApi, profile } from '@/test/api'
import { renderApp } from '@/test/render'

const food = category(1, { name: 'Food', icon: '🍜' })
const transport = category(2, { name: 'Transport', icon: '🚌' })
const salary = category(3, { name: 'Salary', icon: '💼', kind: 'income' })
const bonus = category(4, { name: 'Bonus', icon: '🎁', kind: 'income' })
const old = category(5, { name: 'Old', icon: '', archived: true })

let api: ReturnType<typeof mockApi>
let list: ReturnType<typeof category>[]

beforeEach(() => {
  list = [food, salary, transport, bonus, old]
  api = mockApi({
    'GET /api/me': () => json(200, profile),
    'GET /api/categories': () => json(200, { categories: list }),
  })
})

function openCategories() {
  return renderApp('/settings/categories', { token: 'tok' })
}

function names(group: string): string[] {
  return within(screen.getByRole('list', { name: group }))
    .getAllByRole('listitem')
    .map((li) => li.textContent.replace(/[^\p{L}\s]/gu, '').trim())
}

describe('Categories', () => {
  it('is reached from Settings', async () => {
    const { user, router } = renderApp('/settings', { token: 'tok' })

    await user.click(await screen.findByRole('link', { name: /Categories/ }))

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/settings/categories')
    })
    expect(await screen.findByRole('link', { name: /Settings/ })).toHaveAttribute(
      'href',
      '/settings',
    )
  })

  it('lists Expenses, Income and Archived, each in the user’s order', async () => {
    openCategories()

    expect(await screen.findByTestId('categories-skeleton')).toBeInTheDocument()
    await screen.findByRole('list', { name: 'Expenses' })
    expect(names('Expenses')).toEqual(['Food', 'Transport'])
    expect(names('Income')).toEqual(['Salary', 'Bonus'])
    expect(names('Archived')).toEqual(['Old'])
  })

  it('adds a category', async () => {
    const coffee = category(6, { name: 'Coffee', icon: '☕', kind: 'income' })
    api.on('POST /api/categories', () => {
      list = [...list, coffee]
      return json(201, coffee)
    })
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    const sheet = await screen.findByRole('dialog', { name: 'Add category' })
    await user.type(within(sheet).getByLabelText('Name'), '  Coffee   beans ')
    await user.click(within(sheet).getByRole('radio', { name: 'Income' }))
    await user.type(within(sheet).getByLabelText('Icon'), '☕')
    await user.click(within(sheet).getByRole('button', { name: 'Add category' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Coffee added.')
    expect(api.onlyCall('POST /api/categories').body).toEqual({
      name: 'Coffee beans',
      kind: 'income',
      icon: '☕',
    })
    await waitFor(() => {
      expect(names('Income')).toEqual(['Salary', 'Bonus', 'Coffee'])
    })
  })

  it('checks the name before sending', async () => {
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    const sheet = await screen.findByRole('dialog', { name: 'Add category' })
    await user.click(within(sheet).getByRole('button', { name: 'Add category' }))

    expect(within(sheet).getByLabelText('Name')).toHaveAccessibleDescription(
      expect.stringContaining('Enter a name.'),
    )
    await user.type(within(sheet).getByLabelText('Name'), 'n'.repeat(51))
    await user.click(within(sheet).getByRole('button', { name: 'Add category' }))
    expect(within(sheet).getByLabelText('Name')).toHaveAccessibleDescription(
      expect.stringContaining('Use at most 50 characters.'),
    )
    expect(api.callsTo('POST /api/categories')).toHaveLength(0)
  })

  it('shows a taken name on the name field', async () => {
    api.on('POST /api/categories', () =>
      apiError(409, 'conflict', 'a category with this name already exists'),
    )
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    const sheet = await screen.findByRole('dialog', { name: 'Add category' })
    const name = within(sheet).getByLabelText('Name')
    await user.type(name, 'food')
    await user.click(within(sheet).getByRole('button', { name: 'Add category' }))

    await waitFor(() => {
      expect(name).toHaveAccessibleDescription(
        expect.stringContaining('A category with this name already exists.'),
      )
    })
    expect(within(sheet).queryByRole('alert')).toBeNull()
  })

  it('shows the 200 limit for the whole form', async () => {
    api.on('POST /api/categories', () =>
      apiError(
        409,
        'conflict',
        'the limit of 200 categories is reached; archived categories count too',
      ),
    )
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    const sheet = await screen.findByRole('dialog', { name: 'Add category' })
    await user.type(within(sheet).getByLabelText('Name'), 'Coffee')
    await user.click(within(sheet).getByRole('button', { name: 'Add category' }))

    expect(await within(sheet).findByRole('alert')).toHaveTextContent(
      'The limit of 200 categories is reached',
    )
  })

  it('renames a category, sending only the name', async () => {
    api.on(`PATCH /api/categories/${food.id}`, () => {
      list = list.map((c) => (c.id === food.id ? { ...c, name: 'Meals' } : c))
      return json(200, { ...food, name: 'Meals' })
    })
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: /Food/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Edit category' })
    expect(sheet).toHaveTextContent('Expense category. The type cannot be changed.')
    expect(within(sheet).queryByRole('radio')).toBeNull()
    const name = within(sheet).getByLabelText('Name')
    expect(name).toHaveValue('Food')
    expect(within(sheet).getByLabelText('Icon')).toHaveValue('🍜')
    await user.clear(name)
    await user.type(name, 'Meals')
    await user.click(within(sheet).getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Meals saved.')
    expect(api.onlyCall(`PATCH /api/categories/${food.id}`).body).toEqual({ name: 'Meals' })
    await waitFor(() => {
      expect(names('Expenses')).toEqual(['Meals', 'Transport'])
    })
  })

  it('archives an active category', async () => {
    api.on(`PATCH /api/categories/${transport.id}`, () => {
      list = list.map((c) => (c.id === transport.id ? { ...c, archived: true } : c))
      return json(200, { ...transport, archived: true })
    })
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: /Transport/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Edit category' })
    await user.click(within(sheet).getByRole('button', { name: 'Archive' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Transport archived.')
    expect(api.onlyCall(`PATCH /api/categories/${transport.id}`).body).toEqual({ archived: true })
    await waitFor(() => {
      expect(names('Archived')).toEqual(['Transport', 'Old'])
    })
    expect(names('Expenses')).toEqual(['Food'])
  })

  it('unarchives an archived category', async () => {
    api.on(`PATCH /api/categories/${old.id}`, () => {
      list = [food, salary, transport, bonus, { ...old, archived: false }]
      return json(200, { ...old, archived: false })
    })
    const { user } = openCategories()

    const archived = await screen.findByRole('list', { name: 'Archived' })
    await user.click(within(archived).getByRole('button', { name: /Old/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Edit category' })
    expect(within(sheet).queryByRole('button', { name: 'Archive' })).toBeNull()
    await user.click(within(sheet).getByRole('button', { name: 'Unarchive' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Old is active again')
    expect(api.onlyCall(`PATCH /api/categories/${old.id}`).body).toEqual({ archived: false })
    await waitFor(() => {
      expect(names('Expenses')).toEqual(['Food', 'Transport', 'Old'])
    })
    expect(screen.queryByRole('list', { name: 'Archived' })).toBeNull()
  })

  it('shows why an unarchive is refused', async () => {
    api.on(`PATCH /api/categories/${old.id}`, () =>
      apiError(409, 'conflict', 'a category with this name already exists'),
    )
    const { user } = openCategories()

    const archived = await screen.findByRole('list', { name: 'Archived' })
    await user.click(within(archived).getByRole('button', { name: /Old/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Edit category' })
    await user.click(within(sheet).getByRole('button', { name: 'Unarchive' }))

    expect(await within(sheet).findByRole('alert')).toHaveTextContent(
      'A category with this name already exists.',
    )
  })

  it('reorders with up and down buttons and sends every active id once', async () => {
    api.on('PUT /api/categories/order', () => {
      list = [transport, food, bonus, salary, old]
      return json(200, { categories: list })
    })
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Edit order' }))
    // Up is disabled at the top and down at the bottom of each group.
    expect(screen.getByRole('button', { name: 'Move Food up' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(screen.getByRole('button', { name: 'Move Transport down' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(screen.getByRole('button', { name: 'Move Salary up' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(screen.getByRole('button', { name: 'Move Bonus down' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(screen.getByRole('button', { name: 'Move Food down' })).not.toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(screen.queryByRole('list', { name: 'Archived' })).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Move Transport up' }))
    await user.click(screen.getByRole('button', { name: 'Move Salary down' }))
    expect(names('Expenses')).toEqual(['Transport', 'Food'])
    expect(names('Income')).toEqual(['Bonus', 'Salary'])
    expect(api.callsTo('PUT /api/categories/order')).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: 'Save order' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Order saved.')
    expect(api.onlyCall('PUT /api/categories/order').body).toEqual({
      ids: [transport.id, food.id, bonus.id, salary.id],
    })
    expect(screen.queryByRole('button', { name: 'Move Food up' })).toBeNull()
    expect(names('Expenses')).toEqual(['Transport', 'Food'])
  })

  it('leaves the order alone on Cancel', async () => {
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Edit order' }))
    await user.click(screen.getByRole('button', { name: 'Move Transport up' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(names('Expenses')).toEqual(['Food', 'Transport'])
    expect(api.callsTo('PUT /api/categories/order')).toHaveLength(0)
  })

  it('reloads the list when the order is refused as stale', async () => {
    api.on('PUT /api/categories/order', () => {
      list = [food, transport, salary, bonus, category(7, { name: 'New' }), old]
      return apiError(409, 'conflict', 'the category list has changed; reload and try again')
    })
    const { user } = openCategories()

    await user.click(await screen.findByRole('button', { name: 'Edit order' }))
    await user.click(screen.getByRole('button', { name: 'Move Transport up' }))
    await user.click(screen.getByRole('button', { name: 'Save order' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The category list has changed; reload and try again.',
    )
    await waitFor(() => {
      expect(names('Expenses')).toEqual(['Food', 'Transport', 'New'])
    })
  })
})
