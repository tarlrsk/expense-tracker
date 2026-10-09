import { cn } from 'cn'
import { useId } from 'react'
import { Bar, BarChart, Rectangle, XAxis, YAxis } from 'recharts'
import type { BarShapeProps } from 'recharts'

import type { DashboardTrendMonth } from '@/api/types'
import { Amount } from '@/components/amount'
import { ChartContainer } from '@/components/ui/chart'
import type { ChartConfig } from '@/components/ui/chart'
import { amountParts } from '@/lib/amount'
import { monthPeriodOf, periodLabel, shortMonthLabel } from '@/lib/dates'
import { toSatang } from '@/lib/money'
import { overview as t } from '@/messages/overview'

// The six-month trend of the Overview (ADR-0087, ADR-0091): one column per month, the chosen one
// in --chart-1 and the five before it softened. Only the chosen month's amount is written, in
// full, above the chart; there is no axis and no tooltip. The chart is a picture: a row of real
// buttons lies over the columns, so each month is tappable, reachable by keyboard and read out.

const chartConfig = {
  spent: { label: t.spent, color: 'var(--chart-1)' },
} satisfies ChartConfig

/** The other months' columns: --chart-1 at this opacity. */
const softened = 0.3

export interface SpendingTrendProps {
  /** Six months, oldest first, ending with the chosen month (ADR-0088). */
  trend: DashboardTrendMonth[]
  /** The chosen month, YYYY-MM. */
  month: string
  /** Whether the screen may move to this month (the months the API accepts, ADR-0089). */
  canPick: (month: string) => boolean
  onPick: (month: string) => void
}

export function SpendingTrend({ trend, month, canPick, onPick }: SpendingTrendProps) {
  const headingId = useId()
  const chosen = trend.find((m) => m.month === month)
  // Column heights only: whole satang as a number is exact far beyond any real total.
  const data = trend.map((m) => ({ month: m.month, spent: Number(toSatang(m.spent)) }))

  return (
    <section aria-labelledby={headingId} className="flex flex-col">
      <h3
        id={headingId}
        className="border-b border-border pb-2 text-sm font-semibold text-muted-foreground"
      >
        {t.trend}
      </h3>
      {chosen && (
        <p
          data-testid="trend-chosen"
          className="flex items-baseline justify-between gap-3 pt-4 pb-2"
        >
          <span className="text-muted-foreground">{monthLabel(chosen.month)}</span>
          <Amount value={chosen.spent} />
        </p>
      )}
      <div className="relative h-40">
        <ChartContainer
          aria-hidden="true"
          config={chartConfig}
          className="pointer-events-none absolute inset-x-0 top-0 aspect-auto h-32 border-b border-border"
        >
          <BarChart
            data={data}
            margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
            barCategoryGap="25%"
            accessibilityLayer={false}
          >
            <XAxis dataKey="month" hide />
            <YAxis hide />
            <Bar
              dataKey="spent"
              fill="var(--color-spent)"
              radius={[4, 4, 0, 0]}
              // A month with little spending still shows a sliver, unlike a month with none.
              minPointSize={(value) => ((value ?? 0) > 0 ? 2 : 0)}
              shape={(props: BarShapeProps) => (
                <Rectangle
                  {...props}
                  fillOpacity={data[props.index]?.month === month ? 1 : softened}
                />
              )}
            />
          </BarChart>
        </ChartContainer>
        <ul className="absolute inset-0 grid grid-cols-6">
          {trend.map((m) => {
            const isChosen = m.month === month
            return (
              <li key={m.month} className="flex">
                <button
                  type="button"
                  data-testid="trend-month"
                  aria-label={t.trendMonth(monthLabel(m.month), amountText(m.spent))}
                  aria-current={isChosen ? 'date' : undefined}
                  disabled={!isChosen && !canPick(m.month)}
                  onClick={() => {
                    if (!isChosen) {
                      onPick(m.month)
                    }
                  }}
                  className={cn(
                    'flex flex-1 flex-col items-center justify-end rounded-md pb-1 text-sm select-none',
                    isChosen ? 'font-semibold text-foreground' : 'text-muted-foreground',
                    'disabled:opacity-60',
                  )}
                >
                  {shortMonthLabel(m.month)}
                </button>
              </li>
            )
          })}
        </ul>
      </div>
    </section>
  )
}

/** "October 2026" for a month written YYYY-MM. */
function monthLabel(month: string): string {
  const period = monthPeriodOf(month)
  return period ? periodLabel(period) : month
}

/** "12,450.00" for a screen reader: the same figures the screen shows. */
function amountText(value: string): string {
  const parts = amountParts(value)
  return parts ? `${parts.sign}${parts.baht}.${parts.satang}` : value
}
