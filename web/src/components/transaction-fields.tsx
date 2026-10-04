import { Link } from '@tanstack/react-router'

import type { Category, CategoryKind } from '@/api/types'
import { CategoryPicker } from '@/components/category-picker'
import { ChoiceGroup } from '@/components/choice-group'
import { FormAlert } from '@/components/messages'
import { TextAreaField, TextField } from '@/components/text-field'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { maxTransactionDate, minTransactionDate } from '@/lib/dates'
import { errorText } from '@/lib/errors'
import { categoryChoices } from '@/lib/transaction-form'
import type { FieldErrors, TransactionDraft } from '@/lib/transaction-form'
import { common } from '@/messages/common'
import { transaction as t } from '@/messages/transaction'

export type CategoriesState =
  | { status: 'pending' }
  | { status: 'error'; error: unknown; retry: () => void }
  | { status: 'success'; categories: readonly Category[] }

/**
 * The fields of a transaction, in the order of ADR-0074: amount (number pad), Expense / Income,
 * that kind's categories, date, merchant, note. Every error sits beside its field.
 */
export function TransactionFields({
  draft,
  onChange,
  errors,
  categories,
  keepCategory,
  today,
}: {
  draft: TransactionDraft
  onChange: (patch: Partial<TransactionDraft>) => void
  errors: FieldErrors
  categories: CategoriesState
  /** A transaction's own archived category, offered so it can be kept. */
  keepCategory?: Category
  today: string
}) {
  const all = categories.status === 'success' ? categories.categories : []
  const choices = categoryChoices(all, draft.kind, keepCategory)

  return (
    <>
      <TextField
        label={t.amount}
        name="amount"
        inputMode="decimal"
        autoComplete="off"
        enterKeyHint="next"
        placeholder="0.00"
        inputClassName="h-14 text-display font-semibold tabular-nums"
        value={draft.amount}
        error={errors.amount}
        onChange={(e) => {
          onChange({ amount: e.target.value })
        }}
      />
      <ChoiceGroup<CategoryKind>
        legend={t.kind}
        choices={[
          { value: 'expense', label: t.expense },
          { value: 'income', label: t.income },
        ]}
        value={draft.kind}
        onChange={(kind) => {
          const stays = categoryChoices(all, kind, keepCategory).some(
            (c) => c.id === draft.categoryId,
          )
          onChange({ kind, categoryId: stays ? draft.categoryId : null })
        }}
      />
      {categories.status === 'pending' && <CategoryGridSkeleton />}
      {categories.status === 'error' && (
        <div className="flex flex-col gap-3">
          <FormAlert>
            <p>{t.categoriesLoadError}</p>
            <p>{errorText(categories.error)}</p>
          </FormAlert>
          <Button variant="outline" onClick={categories.retry}>
            {common.tryAgain}
          </Button>
        </div>
      )}
      {categories.status === 'success' && (
        <CategoryPicker
          legend={t.category}
          categories={choices}
          value={draft.categoryId}
          error={errors.category}
          onChange={(categoryId) => {
            onChange({ categoryId })
          }}
          empty={
            <div className="flex flex-col gap-1 rounded-xl bg-muted px-4 py-3">
              <p>{t.noCategories(draft.kind)}</p>
              <Link
                to="/settings/categories"
                className="flex min-h-11 items-center self-start font-semibold text-primary underline underline-offset-4"
              >
                {t.toCategories}
              </Link>
            </div>
          }
        />
      )}
      <TextField
        label={t.date}
        name="date"
        type="date"
        min={minTransactionDate}
        max={maxTransactionDate(today)}
        value={draft.date}
        error={errors.date}
        onChange={(e) => {
          onChange({ date: e.target.value })
        }}
      />
      <TextField
        label={t.merchant}
        hint={t.merchantHint}
        name="merchant"
        autoComplete="off"
        autoCapitalize="words"
        enterKeyHint="next"
        value={draft.merchant}
        error={errors.merchant}
        onChange={(e) => {
          onChange({ merchant: e.target.value })
        }}
      />
      <TextAreaField
        label={t.note}
        hint={t.noteHint}
        name="note"
        autoCapitalize="sentences"
        rows={3}
        value={draft.note}
        error={errors.note}
        onChange={(e) => {
          onChange({ note: e.target.value })
        }}
      />
    </>
  )
}

function CategoryGridSkeleton() {
  return (
    <div className="flex flex-col gap-2" data-testid="categories-skeleton" aria-hidden="true">
      <Skeleton className="h-6 w-24" />
      <div className="grid grid-cols-3 gap-2 sm:grid-cols-4">
        {[0, 1, 2, 3, 4, 5].map((i) => (
          <Skeleton key={i} className="h-20 rounded-xl" />
        ))}
      </div>
    </div>
  )
}
