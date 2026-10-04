import { useId } from 'react'
import type * as React from 'react'

import type { Category } from '@/api/types'
import { transaction as t } from '@/messages/transaction'

/**
 * Choosing one category: a grid of emoji tiles with their names, as native radio buttons, so
 * arrow keys and screen readers work. Each tile is well over 44px high.
 */
export function CategoryPicker({
  legend,
  categories,
  value,
  onChange,
  error,
  empty,
}: {
  legend: string
  /** In the user's order. */
  categories: readonly Category[]
  value: string | null
  onChange: (id: string) => void
  error?: string | null
  /** Shown instead of the grid when there is nothing to choose. */
  empty?: React.ReactNode
}) {
  const name = useId()
  const errorId = `${name}-error`
  return (
    <fieldset
      className="flex min-w-0 flex-col gap-2"
      aria-describedby={error ? errorId : undefined}
    >
      <legend className="mb-2 text-base leading-snug font-semibold">{legend}</legend>
      {categories.length === 0 ? (
        empty
      ) : (
        <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
          {categories.map((category) => (
            <label
              key={category.id}
              className="flex min-h-20 min-w-0 flex-col items-center justify-center gap-1 rounded-xl border border-border bg-card px-1 py-2 text-center select-none has-checked:border-primary has-checked:bg-primary/8 has-checked:ring-2 has-checked:ring-primary has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-ring"
            >
              <input
                type="radio"
                name={name}
                value={category.id}
                checked={value === category.id}
                aria-invalid={error ? true : undefined}
                onChange={() => {
                  onChange(category.id)
                }}
                className="sr-only"
              />
              <span aria-hidden="true" className="text-2xl leading-none">
                {category.icon}
              </span>
              <span className="line-clamp-2 text-sm [overflow-wrap:anywhere]">
                {category.name}
                {category.archived && (
                  <>
                    {' '}
                    <span className="block text-muted-foreground">({t.archived})</span>
                  </>
                )}
              </span>
            </label>
          ))}
        </div>
      )}
      <p id={errorId} aria-live="polite" className="text-sm text-destructive empty:hidden">
        {error ?? ''}
      </p>
    </fieldset>
  )
}
