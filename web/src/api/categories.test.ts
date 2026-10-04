import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, category, json, mockApi } from '@/test/api'

import { createCategory, listCategories, reorderCategories, updateCategory } from './categories'
import { setTokenProvider } from './client'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

const food = category(1, { name: 'Food', icon: '🍜' })
const salary = category(2, { name: 'Salary', icon: '💼', kind: 'income' })
const old = category(3, { name: 'Old', icon: '', archived: true })

describe('listCategories', () => {
  it('gets the list, archived ones included, with the bearer token', async () => {
    api.on('GET /api/categories', () => json(200, { categories: [food, salary, old] }))

    await expect(listCategories()).resolves.toEqual({ categories: [food, salary, old] })

    expect(api.callsTo('GET /api/categories')[0]?.headers.get('Authorization')).toBe('Bearer tok')
  })

  it.each([
    { name: 'no categories key', body: {} },
    { name: 'an unknown kind', body: { categories: [{ ...food, kind: 'transfer' }] } },
    { name: 'a null icon', body: { categories: [{ ...food, icon: null }] } },
    { name: 'a text sort_order', body: { categories: [{ ...food, sort_order: '1' }] } },
    { name: 'a fractional sort_order', body: { categories: [{ ...food, sort_order: 1.5 }] } },
    { name: 'no archived flag', body: { categories: [{ ...food, archived: undefined }] } },
    { name: 'an empty id', body: { categories: [{ ...food, id: '' }] } },
  ])('refuses a list with $name', async ({ body }) => {
    api.on('GET /api/categories', () => json(200, body))

    await expect(listCategories()).rejects.toMatchObject({ code: 'bad_response' })
  })
})

describe('createCategory', () => {
  it('posts the name, kind and icon', async () => {
    const created = category(4, { name: 'Coffee', icon: '☕' })
    api.on('POST /api/categories', () => json(201, created))

    await expect(createCategory({ name: 'Coffee', kind: 'expense', icon: '☕' })).resolves.toEqual(
      created,
    )

    expect(api.onlyCall('POST /api/categories').body).toEqual({
      name: 'Coffee',
      kind: 'expense',
      icon: '☕',
    })
  })

  it('passes a taken name through as conflict', async () => {
    api.on('POST /api/categories', () =>
      apiError(409, 'conflict', 'a category with this name already exists'),
    )

    await expect(createCategory({ name: 'Food', kind: 'expense' })).rejects.toMatchObject({
      code: 'conflict',
      status: 409,
    })
  })
})

describe('updateCategory', () => {
  it('patches only the fields given, on the escaped path', async () => {
    api.on(`PATCH /api/categories/${food.id}`, () => json(200, { ...food, archived: true }))

    await expect(updateCategory(food.id, { archived: true })).resolves.toEqual({
      ...food,
      archived: true,
    })

    expect(api.onlyCall(`PATCH /api/categories/${food.id}`).body).toEqual({ archived: true })
    await updateCategory('a/b', { name: 'x' }).catch(() => undefined)
    expect(api.calls.at(-1)?.path).toBe('/api/categories/a%2Fb')
  })

  it('passes not_found through', async () => {
    api.on(`PATCH /api/categories/${food.id}`, () =>
      apiError(404, 'not_found', 'there is no such category'),
    )

    await expect(updateCategory(food.id, { name: 'x' })).rejects.toMatchObject({
      code: 'not_found',
    })
  })
})

describe('reorderCategories', () => {
  it('puts the ids and returns the new list', async () => {
    api.on('PUT /api/categories/order', () => json(200, { categories: [salary, food, old] }))

    await expect(reorderCategories({ ids: [salary.id, food.id] })).resolves.toEqual({
      categories: [salary, food, old],
    })

    expect(api.onlyCall('PUT /api/categories/order').body).toEqual({ ids: [salary.id, food.id] })
  })

  it('passes a stale list through as conflict', async () => {
    api.on('PUT /api/categories/order', () =>
      apiError(409, 'conflict', 'the category list has changed; reload and try again'),
    )

    await expect(reorderCategories({ ids: [food.id] })).rejects.toMatchObject({
      code: 'conflict',
    })
  })
})
