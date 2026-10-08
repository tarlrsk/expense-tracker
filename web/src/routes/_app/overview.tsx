import { useQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useId, useState } from 'react'

import type { Dashboard, DashboardCategory, DashboardTotals } from '@/api/types'
import { Amount } from '@/components/amount'
import { CategoryIcon } from '@/components/category-icon'
import { PageTitle } from '@/components/layout'
import { FormAlert } from '@/components/messages'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  minTransactionDate,
  monthOf,
  monthParam,
  periodLabel,
  shiftPeriod,
  today as todayIn,
} from '@/lib/dates'
import type { Period } from '@/lib/dates'
import { errorText } from '@/lib/errors'
import { toSatang } from '@/lib/money'
import { barPercent, shareLabel } from '@/lib/share'
import { common } from '@/messages/common'
import { overview as t } from '@/messages/overview'
import { dashboardQuery } from '@/queries/dashboard'

// One month's totals and spending by category (ADR-0087, ADR-0090), from one request to
// GET /api/dashboard. The month is kept on the screen only and starts at this month; the arrows
// stop at the months the API accepts: 2000-01 to one year after this month (ADR-0089).
export const Route = createFileRoute('/_app/overview')({
  component: OverviewPage,
})

function OverviewPage() {
  const today = todayIn()
  const current = monthOf(today)
  const [period, setPeriod] = useState<Period>(current)
  const month = monthParam(period)
  const dashboard = useQuery(dashboardQuery(month))

  const label = periodLabel(period)
  const previous = shiftPeriod(period, -1)
  const next = shiftPeriod(period, 1)
  const canGoPrevious = previous.from >= minTransactionDate
  const canGoNext = monthParam(next) <= monthParam(shiftPeriod(current, 12))

  return (
    <>
      <PageTitle>{t.title}</PageTitle>
      <div className="flex items-center justify-between gap-2">
        <Button
          variant="ghost"
          size="icon"
          aria-label={t.previous}
          disabled={!canGoPrevious}
          focusableWhenDisabled
          onClick={() => {
            setPeriod(previous)
          }}
        >
          <ChevronLeftIcon />
        </Button>
        <h2 aria-live="polite" className="min-w-0 text-center text-xl font-semibold">
          {label}
        </h2>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t.next}
          disabled={!canGoNext}
          focusableWhenDisabled
          onClick={() => {
            setPeriod(next)
          }}
        >
          <ChevronRightIcon />
        </Button>
      </div>

      {dashboard.isPending && <OverviewSkeleton />}
      {dashboard.isError && (
        <div className="flex flex-col gap-4">
          <FormAlert>
            <p>{t.loadError}</p>
            <p>{errorText(dashboard.error)}</p>
          </FormAlert>
          <Button
            variant="outline"
            onClick={() => {
              void dashboard.refetch()
            }}
          >
            {common.tryAgain}
          </Button>
        </div>
      )}
      {dashboard.isSuccess && <MonthOverview data={dashboard.data} label={label} />}
    </>
  )
}

function MonthOverview({ data, label }: { data: Dashboard; label: string }) {
  const { totals, categories } = data
  if (toSatang(totals.spent) === 0n && toSatang(totals.income) === 0n) {
    return (
      <div className="flex flex-col gap-2 border-y border-border py-8">
        <p>{t.empty(label)}</p>
        <p className="text-muted-foreground">{t.emptyNote}</p>
        <Link
          to="/add"
          className="flex min-h-11 items-center self-start font-semibold text-primary underline underline-offset-4"
        >
          {t.toAdd}
        </Link>
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-8">
      <Totals totals={totals} />
      <Breakdown categories={categories} spent={totals.spent} />
    </div>
  )
}

function Totals({ totals }: { totals: DashboardTotals }) {
  const headingId = useId()
  const difference = toSatang(totals.difference)
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <h3 id={headingId} className="sr-only">
        {t.totals}
      </h3>
      <dl className="flex flex-col gap-4">
        <div data-testid="total-spent" className="flex flex-col gap-1">
          <dt className="text-sm text-muted-foreground">{t.spent}</dt>
          <dd>
            <Amount value={totals.spent} size="display" />
          </dd>
        </div>
        <div className="grid grid-cols-2 gap-4 border-t border-border pt-4">
          <div data-testid="total-income" className="flex min-w-0 flex-col gap-1">
            <dt className="text-sm text-muted-foreground">{t.income}</dt>
            <dd>
              <Amount
                value={totals.income}
                variant={toSatang(totals.income) > 0n ? 'income' : 'expense'}
              />
            </dd>
          </div>
          <div data-testid="total-difference" className="flex min-w-0 flex-col gap-1">
            <dt className="text-sm text-muted-foreground">{t.difference}</dt>
            <dd>
              {/* Above zero: green with a "+"; below: plain ink with a minus (ADR-0090). */}
              <Amount value={totals.difference} variant={difference > 0n ? 'income' : 'expense'} />
            </dd>
          </div>
        </div>
      </dl>
    </section>
  )
}

function Breakdown({ categories, spent }: { categories: DashboardCategory[]; spent: string }) {
  const headingId = useId()
  // The list is largest first, so the first row is the full-width bar.
  const largest = categories[0]?.total ?? '0.00'
  return (
    <section aria-labelledby={headingId} className="flex flex-col">
      <h3
        id={headingId}
        className="border-b border-border pb-2 text-sm font-semibold text-muted-foreground"
      >
        {t.breakdown}
      </h3>
      {categories.length === 0 ? (
        <p className="py-4 text-muted-foreground">{t.noSpending}</p>
      ) : (
        <ul className="flex flex-col divide-y divide-border">
          {categories.map((row) => (
            <li key={row.category_id}>
              <CategoryRow
                row={row}
                share={shareLabel(row.total, spent) ?? '0%'}
                width={barPercent(row.total, largest)}
              />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function CategoryRow({
  row,
  share,
  width,
}: {
  row: DashboardCategory
  share: string
  width: number
}) {
  return (
    <div data-testid="category-row" className="flex min-h-14 items-center gap-3 py-3">
      <CategoryIcon icon={row.icon} />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <div className="flex items-baseline justify-between gap-3">
          <span className="flex min-w-0 flex-col">
            <span className="break-words">{row.name}</span>
            {row.archived && <span className="text-sm text-muted-foreground">{t.archived}</span>}
          </span>
          <Amount value={row.total} />
        </div>
        <div className="flex items-center gap-3">
          <div aria-hidden="true" className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              data-testid="category-bar"
              className="h-full rounded-full bg-chart-1"
              style={{ width: `${String(width)}%` }}
            />
          </div>
          <span className="w-10 shrink-0 text-right text-sm text-muted-foreground tabular-nums">
            <span aria-hidden="true">{share}</span>
            <span className="sr-only">{t.share(share)}</span>
          </span>
        </div>
      </div>
    </div>
  )
}

function OverviewSkeleton() {
  return (
    <div aria-hidden="true" data-testid="overview-skeleton" className="flex flex-col gap-8">
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-8 w-40" />
        </div>
        <div className="grid grid-cols-2 gap-4 border-t border-border pt-4">
          {[0, 1].map((n) => (
            <div key={n} className="flex flex-col gap-2">
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-5 w-24" />
            </div>
          ))}
        </div>
      </div>
      <div className="flex flex-col">
        <Skeleton className="mb-2 h-4 w-40" />
        {[0, 1, 2].map((n) => (
          <div key={n} className="flex items-center gap-3 border-t border-border py-3">
            <Skeleton className="size-10 rounded-full" />
            <div className="flex flex-1 flex-col gap-2">
              <Skeleton className="h-5 w-2/3" />
              <Skeleton className="h-1.5 w-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
