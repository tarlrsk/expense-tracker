import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { SubmitEvent } from 'react'

import { deleteTransaction, updateTransaction } from '@/api/transactions'
import type { Category, Transaction, UpdateTransactionRequest } from '@/api/types'
import { Amount } from '@/components/amount'
import { FormAlert } from '@/components/messages'
import { TransactionFields } from '@/components/transaction-fields'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { amountParts } from '@/lib/amount'
import { longDayLabel, today as todayIn } from '@/lib/dates'
import { errorText, isApiError } from '@/lib/errors'
import {
  apiFieldMessages,
  categoryChoices,
  changedFields,
  checkTransaction,
  fieldOfApiError,
} from '@/lib/transaction-form'
import type { FieldErrors, TransactionDraft } from '@/lib/transaction-form'
import { common } from '@/messages/common'
import { transaction as t } from '@/messages/transaction'
import { transactionsKey } from '@/queries/money'

/**
 * One transaction from History, in a bottom sheet (ADR-0074): it shows the row already loaded
 * (there is no "get one" endpoint) and lets the user change or delete it. `onDone` gets the
 * notice to show once the sheet closes.
 */
export function TransactionSheet({
  transaction,
  categories,
  open,
  onOpenChange,
  onDone,
}: {
  /** Kept while the sheet closes, so its text does not vanish mid-animation. */
  transaction: Transaction | null
  categories: readonly Category[]
  open: boolean
  onOpenChange: (open: boolean) => void
  onDone: (notice: string | null) => void
}) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        {transaction && (
          <TransactionDetail transaction={transaction} categories={categories} onDone={onDone} />
        )}
      </SheetContent>
    </Sheet>
  )
}

function TransactionDetail({
  transaction,
  categories,
  onDone,
}: {
  transaction: Transaction
  categories: readonly Category[]
  onDone: (notice: string | null) => void
}) {
  const current = categories.find((c) => c.id === transaction.category_id)
  const [confirming, setConfirming] = useState(false)
  const title =
    transaction.merchant !== '' ? transaction.merchant : (current?.name ?? t.unknownCategory)
  const variant = current?.kind === 'income' ? 'income' : 'expense'

  if (confirming) {
    return (
      <DeleteConfirm
        transaction={transaction}
        variant={variant}
        onCancel={() => {
          setConfirming(false)
        }}
        onDone={onDone}
      />
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle className="[overflow-wrap:anywhere]">{title}</SheetTitle>
        <SheetDescription>
          {longDayLabel(transaction.occurred_on)} · {current?.name ?? t.unknownCategory}
        </SheetDescription>
        <Amount value={transaction.amount} variant={variant} size="display" />
      </SheetHeader>
      <EditForm
        transaction={transaction}
        categories={categories}
        current={current}
        onDone={onDone}
        onDelete={() => {
          setConfirming(true)
        }}
      />
    </div>
  )
}

function EditForm({
  transaction,
  categories,
  current,
  onDone,
  onDelete,
}: {
  transaction: Transaction
  categories: readonly Category[]
  current: Category | undefined
  onDone: (notice: string | null) => void
  onDelete: () => void
}) {
  const queryClient = useQueryClient()
  const today = todayIn()
  const [draft, setDraft] = useState<TransactionDraft>(() => ({
    amount: transaction.amount,
    kind: current?.kind ?? 'expense',
    categoryId: transaction.category_id,
    date: transaction.occurred_on,
    merchant: transaction.merchant,
    note: transaction.note,
  }))
  const [errors, setErrors] = useState<FieldErrors>({})

  const save = useMutation({
    mutationFn: (changes: UpdateTransactionRequest) => updateTransaction(transaction.id, changes),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: transactionsKey })
      onDone(t.saved)
    },
    onError: (err) => {
      if (isApiError(err, 'not_found')) {
        void queryClient.invalidateQueries({ queryKey: transactionsKey })
        return
      }
      if (isApiError(err, 'invalid_input')) {
        const field = fieldOfApiError(err.message)
        if (field) {
          setErrors({ [field]: apiFieldMessages[field] })
        }
      }
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (save.isPending) {
      return
    }
    const choices = categoryChoices(categories, draft.kind, current)
    const result = checkTransaction(draft, { today, categoryIds: choices.map((c) => c.id) })
    if (!result.ok) {
      setErrors(result.errors)
      return
    }
    setErrors({})
    const changes = changedFields(transaction, result.value)
    if (Object.keys(changes).length === 0) {
      // Nothing to send: the API would write nothing either.
      onDone(null)
      return
    }
    save.mutate(changes)
  }

  const formError = !save.isError
    ? null
    : isApiError(save.error, 'not_found')
      ? t.gone
      : isApiError(save.error, 'invalid_input') && fieldOfApiError(save.error.message) !== null
        ? null
        : errorText(save.error)

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <TransactionFields
        draft={draft}
        onChange={(patch) => {
          setDraft((d) => ({ ...d, ...patch }))
        }}
        errors={errors}
        categories={{ status: 'success', categories }}
        keepCategory={current}
        today={today}
      />
      {formError && <FormAlert>{formError}</FormAlert>}
      <SheetFooter className="mt-0">
        <Button type="button" variant="destructive-ghost" size="lg" onClick={onDelete}>
          {t.delete}
        </Button>
        <Button type="submit" size="lg" disabled={save.isPending} focusableWhenDisabled>
          {save.isPending ? t.saving : t.saveChanges}
        </Button>
      </SheetFooter>
    </form>
  )
}

function DeleteConfirm({
  transaction,
  variant,
  onCancel,
  onDone,
}: {
  transaction: Transaction
  variant: 'expense' | 'income'
  onCancel: () => void
  onDone: (notice: string | null) => void
}) {
  const queryClient = useQueryClient()
  const remove = useMutation({
    mutationFn: () => deleteTransaction(transaction.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: transactionsKey })
      onDone(t.deleted)
    },
    onError: (err) => {
      if (isApiError(err, 'not_found')) {
        void queryClient.invalidateQueries({ queryKey: transactionsKey })
      }
    },
  })
  const parts = amountParts(transaction.amount)
  const amountText = parts
    ? `${variant === 'income' ? '+' : ''}${parts.baht}.${parts.satang}`
    : transaction.amount

  return (
    <div className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.deleteTitle}</SheetTitle>
        <SheetDescription>
          {t.deleteWarning(amountText, longDayLabel(transaction.occurred_on))}
        </SheetDescription>
      </SheetHeader>
      {remove.isError && (
        <FormAlert>
          {isApiError(remove.error, 'not_found') ? t.gone : errorText(remove.error)}
        </FormAlert>
      )}
      <SheetFooter className="mt-0">
        <Button variant="outline" size="lg" onClick={onCancel}>
          {common.cancel}
        </Button>
        <Button
          variant="destructive"
          size="lg"
          disabled={remove.isPending}
          focusableWhenDisabled
          onClick={() => {
            remove.mutate()
          }}
        >
          {remove.isPending ? t.deleting : t.deleteSubmit}
        </Button>
      </SheetFooter>
    </div>
  )
}
