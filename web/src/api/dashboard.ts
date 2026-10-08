// The month's totals for the Overview screen: GET /api/dashboard (docs/04-api.md, ADR-0089).
// Amounts stay text; nothing here turns one into a number.

import { apiRequest } from './client'
import { amount, array, boolean, integer, month, nonEmptyString, object, string } from './shape'
import type {
  Dashboard,
  DashboardCategory,
  DashboardDescription,
  DashboardTotals,
  DashboardTrendMonth,
} from './types'

function parseTotals(value: unknown): DashboardTotals {
  const o = object(value, 'totals')
  return {
    spent: amount(o, 'spent'),
    income: amount(o, 'income'),
    difference: amount(o, 'difference'),
  }
}

function parseCategory(value: unknown): DashboardCategory {
  const o = object(value, 'category')
  return {
    category_id: nonEmptyString(o, 'category_id'),
    name: string(o, 'name'),
    icon: string(o, 'icon'),
    archived: boolean(o, 'archived'),
    total: amount(o, 'total'),
    count: integer(o, 'count'),
  }
}

function parseTrendMonth(value: unknown): DashboardTrendMonth {
  const o = object(value, 'trend month')
  return { month: month(o, 'month'), spent: amount(o, 'spent') }
}

function parseDescription(value: unknown): DashboardDescription {
  const o = object(value, 'description')
  return {
    description: string(o, 'description'),
    total: amount(o, 'total'),
    count: integer(o, 'count'),
  }
}

export function parseDashboard(body: unknown): Dashboard {
  const o = object(body, 'dashboard')
  return {
    month: month(o, 'month'),
    totals: parseTotals(o.totals),
    categories: array(o.categories, 'categories').map(parseCategory),
    trend: array(o.trend, 'trend').map(parseTrendMonth),
    top_descriptions: array(o.top_descriptions, 'top_descriptions').map(parseDescription),
  }
}

/** GET /api/dashboard?month=YYYY-MM — `month` from 2000-01 to one year after this month. */
export function getDashboard(monthParam: string, signal?: AbortSignal): Promise<Dashboard> {
  const q = new URLSearchParams({ month: monthParam })
  return apiRequest(`/dashboard?${q.toString()}`, { signal, parse: parseDashboard })
}
