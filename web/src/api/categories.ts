// The user's categories: /api/categories (docs/04-api.md, ADR-0061, ADR-0069).

import { apiRequest } from './client'
import { array, boolean, integer, nonEmptyString, object, oneOf, string, timestamp } from './shape'
import { categoryKinds } from './types'
import type {
  Category,
  CreateCategoryRequest,
  ListCategoriesResponse,
  ReorderCategoriesRequest,
  UpdateCategoryRequest,
} from './types'

export function parseCategory(value: unknown): Category {
  const o = object(value, 'category')
  return {
    id: nonEmptyString(o, 'id'),
    name: string(o, 'name'),
    icon: string(o, 'icon'),
    kind: oneOf(o, 'kind', categoryKinds),
    archived: boolean(o, 'archived'),
    sort_order: integer(o, 'sort_order'),
    created_at: timestamp(o, 'created_at'),
    updated_at: timestamp(o, 'updated_at'),
  }
}

export function parseListCategories(body: unknown): ListCategoriesResponse {
  const o = object(body, 'list')
  return { categories: array(o.categories, 'categories').map(parseCategory) }
}

/** GET /api/categories — archived ones included, in the user's order. */
export function listCategories(signal?: AbortSignal): Promise<ListCategoriesResponse> {
  return apiRequest('/categories', { signal, parse: parseListCategories })
}

/** POST /api/categories — 409 `conflict` for a taken name or at 200 categories. */
export function createCategory(req: CreateCategoryRequest): Promise<Category> {
  return apiRequest('/categories', { method: 'POST', body: req, parse: parseCategory })
}

/**
 * PATCH /api/categories/{id} — rename, set the icon, archive or unarchive. Unarchiving is 409
 * `conflict` when an active category has the name now.
 */
export function updateCategory(id: string, req: UpdateCategoryRequest): Promise<Category> {
  return apiRequest(`/categories/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: req,
    parse: parseCategory,
  })
}

/**
 * PUT /api/categories/order — `ids` must name every non-archived category once; otherwise 409
 * `conflict` and nothing changes. Returns the whole list.
 */
export function reorderCategories(req: ReorderCategoriesRequest): Promise<ListCategoriesResponse> {
  return apiRequest('/categories/order', {
    method: 'PUT',
    body: req,
    parse: parseListCategories,
  })
}
