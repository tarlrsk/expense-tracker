// The name and icon rules of a category (ADR-0069), checked before sending so each error sits
// beside its field. The API applies them again.

import { categories as t } from '@/messages/categories'

import { characterCount } from './password'

export const maxCategoryNameLength = 50
export const maxCategoryIconLength = 32
/** How many categories one user may have, archived ones included (ADR-0069). */
export const maxCategories = 200

const controlPattern = /\p{Cc}/u

/** The name as the API stores it: trimmed, every inner run of whitespace one space. */
export function normalizeCategoryName(name: string): string {
  return name.trim().replace(/\s+/gu, ' ')
}

export type TextCheck = { ok: true; value: string } | { ok: false; error: string }

export function checkCategoryName(name: string): TextCheck {
  const value = normalizeCategoryName(name)
  if (value === '') {
    return { ok: false, error: t.enterName }
  }
  if (characterCount(value) > maxCategoryNameLength) {
    return { ok: false, error: t.nameTooLong }
  }
  if (controlPattern.test(value)) {
    return { ok: false, error: t.specialCharacters }
  }
  return { ok: true, value }
}

/** An icon is optional free text, such as one emoji; empty means none. */
export function checkCategoryIcon(icon: string): TextCheck {
  const value = icon.trim()
  if (characterCount(value) > maxCategoryIconLength) {
    return { ok: false, error: t.iconTooLong }
  }
  if (controlPattern.test(value)) {
    return { ok: false, error: t.specialCharacters }
  }
  return { ok: true, value }
}

/** Moves the item at `index` one place up (-1) or down (+1); out of range changes nothing. */
export function moveItem<T>(items: readonly T[], index: number, step: -1 | 1): T[] {
  const target = index + step
  const copy = [...items]
  const a = copy[index]
  const b = copy[target]
  if (a === undefined || b === undefined) {
    return copy
  }
  copy[index] = b
  copy[target] = a
  return copy
}
