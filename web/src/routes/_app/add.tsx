import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import type { SubmitEvent } from 'react'

import { createTransaction } from '@/api/transactions'
import type { CreateTransactionRequest } from '@/api/types'
import { ChoiceGroup } from '@/components/choice-group'
import { ActionBar, FormSheet, PageTitle } from '@/components/layout'
import { FormAlert, Notice } from '@/components/messages'
import { QuickEntry } from '@/components/quick-entry'
import { TransactionFields } from '@/components/transaction-fields'
import type { CategoriesState } from '@/components/transaction-fields'
import { Button } from '@/components/ui/button'
import { today as todayIn } from '@/lib/dates'
import { errorText, isApiError } from '@/lib/errors'
import {
  apiFieldMessages,
  categoryChoices,
  checkTransaction,
  fieldOfApiError,
} from '@/lib/transaction-form'
import type { FieldErrors, TransactionDraft } from '@/lib/transaction-form'
import { uuidv7 } from '@/lib/uuid'
import { add as t } from '@/messages/add'
import { entry } from '@/messages/entry'
import { categoriesQuery, transactionsKey } from '@/queries/money'

// Recording expenses and income. Two ways, one switch (ADR-0076): Quick, a typed text read by
// POST /api/entry/parse and confirmed in a sheet; and Form, one transaction at a time
// (ADR-0074), which starts again after each save and stays on this screen.
export const Route = createFileRoute('/_app/add')({
  component: AddPage,
})

type AddMode = 'quick' | 'form'

/** The localStorage key of the last mode used; device-only, not account data. */
const addModeStorageKey = 'satang.add-mode'

function readMode(): AddMode {
  try {
    return localStorage.getItem(addModeStorageKey) === 'form' ? 'form' : 'quick'
  } catch {
    return 'quick'
  }
}

function rememberMode(mode: AddMode): void {
  try {
    localStorage.setItem(addModeStorageKey, mode)
  } catch {
    // Storage is unavailable: the mode is simply not remembered.
  }
}

function AddPage() {
  const [mode, setMode] = useState(readMode)
  const [notice, setNotice] = useState<string | null>(null)

  return (
    <>
      <PageTitle>{t.title}</PageTitle>
      <ChoiceGroup<AddMode>
        legend={entry.mode}
        choices={[
          { value: 'quick', label: entry.quick },
          { value: 'form', label: entry.form },
        ]}
        value={mode}
        onChange={(next) => {
          rememberMode(next)
          setNotice(null)
          setMode(next)
        }}
      />
      <Notice>{notice}</Notice>
      {mode === 'quick' ? <QuickEntry onNotice={setNotice} /> : <ManualForm onNotice={setNotice} />}
    </>
  )
}

function emptyDraft(today: string): TransactionDraft {
  return { amount: '', kind: 'expense', categoryId: null, date: today, merchant: '', note: '' }
}

/** The manual form: one transaction; `onNotice` says "Saved." above the switch. */
function ManualForm({ onNotice }: { onNotice: (text: string | null) => void }) {
  const queryClient = useQueryClient()
  const categories = useQuery(categoriesQuery)
  const today = todayIn()
  const [draft, setDraft] = useState(() => emptyDraft(today))
  // The id of the transaction being entered. It changes only after a save succeeds, so a retry
  // after a failed send reuses it and the API saves once (ADR-0040).
  const [id, setId] = useState(() => uuidv7())
  const [errors, setErrors] = useState<FieldErrors>({})

  const save = useMutation({
    mutationFn: createTransaction,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: transactionsKey })
      setDraft(emptyDraft(todayIn()))
      setId(uuidv7())
      setErrors({})
      onNotice(t.saved)
    },
    onError: (err) => {
      if (!isApiError(err, 'invalid_input')) {
        return
      }
      const field = fieldOfApiError(err.message)
      if (field) {
        setErrors({ [field]: apiFieldMessages[field] })
      }
      if (field === 'category') {
        // The category may have been archived elsewhere; show the current list.
        void queryClient.invalidateQueries({ queryKey: categoriesQuery.queryKey })
      }
    },
  })

  const categoriesState: CategoriesState = categories.isSuccess
    ? { status: 'success', categories: categories.data.categories }
    : categories.isError
      ? {
          status: 'error',
          error: categories.error,
          retry: () => {
            void categories.refetch()
          },
        }
      : { status: 'pending' }

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (save.isPending) {
      return
    }
    onNotice(null)
    const choices = categoryChoices(categories.data?.categories ?? [], draft.kind)
    const result = checkTransaction(draft, { today, categoryIds: choices.map((c) => c.id) })
    if (!result.ok) {
      setErrors(result.errors)
      return
    }
    setErrors({})
    const { merchant, note, ...required } = result.value
    const req: CreateTransactionRequest = { id, ...required }
    if (merchant !== '') {
      req.merchant = merchant
    }
    if (note !== '') {
      req.note = note
    }
    save.mutate(req)
  }

  const formError =
    save.isError &&
    !(isApiError(save.error, 'invalid_input') && fieldOfApiError(save.error.message) !== null)
      ? errorText(save.error)
      : null

  return (
    <form noValidate onSubmit={submit} className="flex flex-1 flex-col gap-6">
      <FormSheet>
        <TransactionFields
          draft={draft}
          onChange={(patch) => {
            setDraft((d) => ({ ...d, ...patch }))
          }}
          errors={errors}
          categories={categoriesState}
          today={today}
        />
      </FormSheet>
      {formError && <FormAlert>{formError}</FormAlert>}
      <ActionBar>
        <Button
          type="submit"
          size="lg"
          className="w-full shadow-[0_4px_16px_rgb(31_122_84/0.3)] md:shadow-none"
          disabled={save.isPending}
          focusableWhenDisabled
        >
          {save.isPending ? t.saving : t.save}
        </Button>
      </ActionBar>
    </form>
  )
}
