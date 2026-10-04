import { describe, expect, it } from 'vitest'

import { checkCategoryIcon, checkCategoryName, moveItem } from './category-form'

describe('checkCategoryName', () => {
  it.each([
    { name: 'Food', value: 'Food' },
    { name: '  Eating   out\t\ttoday ', value: 'Eating out today' },
    { name: 'อาหาร', value: 'อาหาร' },
    { name: 'n'.repeat(50), value: 'n'.repeat(50) },
    { name: '🍜'.repeat(50), value: '🍜'.repeat(50) },
  ])('takes $name', ({ name, value }) => {
    expect(checkCategoryName(name)).toEqual({ ok: true, value })
  })

  it.each([
    { name: '', error: 'Enter a name.' },
    { name: '   ', error: 'Enter a name.' },
    { name: 'n'.repeat(51), error: 'Use at most 50 characters.' },
    { name: 'a\u0007b', error: 'Remove tabs and other special characters.' },
  ])('refuses $name', ({ name, error }) => {
    expect(checkCategoryName(name)).toEqual({ ok: false, error })
  })
})

describe('checkCategoryIcon', () => {
  it.each([
    { icon: '', value: '' },
    { icon: ' 🍜 ', value: '🍜' },
    { icon: '👨‍👩‍👧', value: '👨‍👩‍👧' },
    { icon: 'x'.repeat(32), value: 'x'.repeat(32) },
  ])('takes $icon', ({ icon, value }) => {
    expect(checkCategoryIcon(icon)).toEqual({ ok: true, value })
  })

  it.each([
    { icon: 'x'.repeat(33), error: 'Use at most 32 characters.' },
    { icon: 'a\tb', error: 'Remove tabs and other special characters.' },
  ])('refuses $icon', ({ icon, error }) => {
    expect(checkCategoryIcon(icon)).toEqual({ ok: false, error })
  })
})

describe('moveItem', () => {
  it.each([
    { items: ['a', 'b', 'c'], index: 1, step: -1 as const, want: ['b', 'a', 'c'] },
    { items: ['a', 'b', 'c'], index: 1, step: 1 as const, want: ['a', 'c', 'b'] },
    { items: ['a', 'b', 'c'], index: 0, step: -1 as const, want: ['a', 'b', 'c'] },
    { items: ['a', 'b', 'c'], index: 2, step: 1 as const, want: ['a', 'b', 'c'] },
  ])('moves $index by $step', ({ items, index, step, want }) => {
    expect(moveItem(items, index, step)).toEqual(want)
  })
})
