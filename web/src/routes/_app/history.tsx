import { useQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useId, useState } from 'react'

import type { Category, Transaction } from '@/api/types'
import { Amount } from '@/components/amount'
import { CategoryIcon } from '@/components/category-icon'
import { ChoiceGroup } from '@/components/choice-group'
import { PageTitle } from '@/components/layout'
import { FormAlert, Notice } from '@/components/messages'
import { TransactionSheet } from '@/components/transaction-sheet'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  dayLabel,
  maxTransactionDate,
  periodContains,
  periodLabel,
  periodOf,
  shiftPeriod,
  today as todayIn,
} from '@/lib/dates'
import type { HistoryView, Period } from '@/lib/dates'
import { errorText } from '@/lib/errors'
import { sumAmounts } from '@/lib/money'
import { common } from '@/messages/common'
import { history as t } from '@/messages/history'
import { transaction as tx } from '@/messages/transaction'
import { categoriesQuery, periodTransactionsQuery } from '@/queries/money'

// A week or a month of transactions, the days as groups inside, newest first (ADR-0071,
// ADR-0072). It opens on Month; the last view chosen is remembered on this device (ADR-0074).
export const Route = createFileRoute('/_app/history')({
  component: HistoryPage,
})

/** The localStorage key of the remembered view; device-only, not account data. */
const historyViewStorageKey = 'satang.history-view'

function readView(): HistoryView {
  try {
    const stored = localStorage.getItem(historyViewStorageKey)
    return stored === 'week' ? 'week' : 'month'
  } catch {
    return 'month'
  }
}

function rememberView(view: HistoryView): void {
  try {
    localStorage.setItem(historyViewStorageKey, view)
  } catch {
    // Storage is unavailable: the view is simply not remembered.
  }
}

interface DayGroup {
  date: string
  transactions: Transaction[]
  expenses: string
  income: string
}

/** Groups rows by day, newest day first, with each day's expense and income totals. */
function groupByDay(
  transactions: readonly Transaction[],
  categories: ReadonlyMap<string, Category>,
): DayGroup[] {
  const byDay = new Map<string, Transaction[]>()
  for (const row of transactions) {
    const day = byDay.get(row.occurred_on)
    if (day) {
      day.push(row)
    } else {
      byDay.set(row.occurred_on, [row])
    }
  }
  return [...byDay.entries()]
    .sort(([a], [b]) => (a < b ? 1 : a > b ? -1 : 0))
    .map(([date, rows]) => {
      const isIncome = (row: Transaction) => categories.get(row.category_id)?.kind === 'income'
      return {
        date,
        transactions: rows,
        expenses: sumAmounts(rows.filter((r) => !isIncome(r)).map((r) => r.amount)),
        income: sumAmounts(rows.filter(isIncome).map((r) => r.amount)),
      }
    })
}

function HistoryPage() {
  const today = todayIn()
  const [period, setPeriod] = useState<Period>(() => periodOf(readView(), today))
  const categories = useQuery(categoriesQuery)
  const transactions = useQuery(periodTransactionsQuery(period))
  const [notice, setNotice] = useState<string | null>(null)
  const [selected, setSelected] = useState<Transaction | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [sheetKey, setSheetKey] = useState(0)

  const label = periodLabel(period)
  const next = shiftPeriod(period, 1)
  const canGoNext = next.from <= maxTransactionDate(today)

  function chooseView(view: HistoryView) {
    rememberView(view)
    setNotice(null)
    setPeriod(periodOf(view, periodContains(period, today) ? today : period.from))
  }

  function move(steps: number) {
    setNotice(null)
    const moved = shiftPeriod(period, steps)
    setPeriod(periodContains(moved, today) ? periodOf(moved.view, today) : moved)
  }

  const categoryById = new Map((categories.data?.categories ?? []).map((c) => [c.id, c]))

  return (
    <>
      <PageTitle>{t.title}</PageTitle>
      <div className="flex flex-col gap-3">
        <ChoiceGroup<HistoryView>
          legend={t.view}
          choices={[
            { value: 'week', label: t.week },
            { value: 'month', label: t.month },
          ]}
          value={period.view}
          onChange={chooseView}
        />
        <div className="flex items-center justify-between gap-2">
          <Button
            variant="ghost"
            size="icon"
            aria-label={t.previous(period.view)}
            onClick={() => {
              move(-1)
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
            aria-label={t.next(period.view)}
            disabled={!canGoNext}
            focusableWhenDisabled
            onClick={() => {
              move(1)
            }}
          >
            <ChevronRightIcon />
          </Button>
        </div>
      </div>
      <Notice>{notice}</Notice>

      {(categories.isPending || transactions.isPending) &&
        !categories.isError &&
        !transactions.isError && <HistorySkeleton />}
      {(categories.isError || transactions.isError) && (
        <div className="flex flex-col gap-4">
          <FormAlert>
            <p>{t.loadError}</p>
            <p>{errorText(categories.error ?? transactions.error)}</p>
          </FormAlert>
          <Button
            variant="outline"
            onClick={() => {
              if (categories.isError) {
                void categories.refetch()
              }
              if (transactions.isError) {
                void transactions.refetch()
              }
            }}
          >
            {common.tryAgain}
          </Button>
        </div>
      )}
      {categories.isSuccess &&
        transactions.isSuccess &&
        (transactions.data.length === 0 ? (
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
        ) : (
          <div className="flex flex-col gap-6">
            {groupByDay(transactions.data, categoryById).map((group) => (
              <DaySection
                key={group.date}
                group={group}
                categories={categoryById}
                onOpen={(row) => {
                  setNotice(null)
                  setSheetKey((k) => k + 1)
                  setSelected(row)
                  setSheetOpen(true)
                }}
              />
            ))}
          </div>
        ))}

      <TransactionSheet
        key={sheetKey}
        transaction={selected}
        categories={categories.data?.categories ?? []}
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        onDone={(text) => {
          setSheetOpen(false)
          setNotice(text)
        }}
      />
    </>
  )
}

function DaySection({
  group,
  categories,
  onOpen,
}: {
  group: DayGroup
  categories: ReadonlyMap<string, Category>
  onOpen: (row: Transaction) => void
}) {
  const headingId = useId()
  return (
    <section aria-labelledby={headingId} className="flex flex-col">
      <div className="flex items-baseline justify-between gap-3 border-b border-border pb-2">
        <h3 id={headingId} className="text-sm font-semibold text-muted-foreground">
          {dayLabel(group.date)}
        </h3>
        <p className="flex flex-wrap items-baseline justify-end gap-x-3">
          <span data-testid="day-expenses">
            <span className="sr-only">{t.spent} </span>
            <Amount value={group.expenses} />
          </span>
          {group.income !== '0.00' && (
            <span data-testid="day-income">
              <span className="sr-only">{t.received} </span>
              <Amount value={group.income} variant="income" />
            </span>
          )}
        </p>
      </div>
      <ul className="flex flex-col divide-y divide-border">
        {group.transactions.map((row) => (
          <li key={row.id}>
            <TransactionRow
              row={row}
              category={categories.get(row.category_id)}
              onOpen={() => {
                onOpen(row)
              }}
            />
          </li>
        ))}
      </ul>
    </section>
  )
}

function TransactionRow({
  row,
  category,
  onOpen,
}: {
  row: Transaction
  category: Category | undefined
  onOpen: () => void
}) {
  const name = category?.name ?? tx.unknownCategory
  return (
    <button
      type="button"
      onClick={onOpen}
      className="flex min-h-14 w-full items-center gap-3 py-3 text-left select-none active:bg-muted"
    >
      <CategoryIcon icon={category?.icon ?? ''} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate">{row.merchant || name}</span>
        {row.merchant && <span className="truncate text-sm text-muted-foreground">{name}</span>}
      </span>
      <Amount value={row.amount} variant={category?.kind === 'income' ? 'income' : 'expense'} />
    </button>
  )
}

function HistorySkeleton() {
  return (
    <div aria-hidden="true" data-testid="history-skeleton" className="flex flex-col gap-6">
      {[0, 1].map((group) => (
        <div key={group} className="flex flex-col">
          <div className="flex justify-between border-b border-border pb-2">
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-5 w-16" />
          </div>
          {[0, 1, 2].map((row) => (
            <div key={row} className="flex items-center gap-3 py-3">
              <Skeleton className="size-10 rounded-full" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-5 w-2/3" />
                <Skeleton className="h-4 w-1/3" />
              </div>
              <Skeleton className="h-5 w-16" />
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
