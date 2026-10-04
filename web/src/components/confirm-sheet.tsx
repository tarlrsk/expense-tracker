import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleAlertIcon, InfoIcon, XIcon } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'

import { createTransaction } from '@/api/transactions'
import type { Category, CreateTransactionRequest, EntryAIStatus } from '@/api/types'
import { Amount } from '@/components/amount'
import { CategoryIcon } from '@/components/category-icon'
import { FormAlert } from '@/components/messages'
import { TransactionFields } from '@/components/transaction-fields'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { dayLabel, isDate, today as todayIn } from '@/lib/dates'
import { errorText, isApiError } from '@/lib/errors'
import { parseAmountInput } from '@/lib/money'
import {
  checkProposal,
  proposalTitle,
  splitProposals,
  textTransactionRequest,
} from '@/lib/quick-entry'
import type { Proposal, ProposalSplit } from '@/lib/quick-entry'
import { apiFieldMessages, fieldOfApiError } from '@/lib/transaction-form'
import type { FieldErrors, TransactionDraft, TransactionField } from '@/lib/transaction-form'
import { common } from '@/messages/common'
import { entry as t } from '@/messages/entry'
import { categoriesQuery, transactionsKey } from '@/queries/money'

/** One parse, shown in the sheet; a new `key` starts the sheet afresh. */
export interface EntrySession {
  key: number
  ai: EntryAIStatus
  proposals: Proposal[]
}

/** What a save attempt left on a row: errors beside its fields, or one for the whole row. */
interface RowState {
  errors: FieldErrors
  failure: string | null
}

interface SaveResult {
  id: string
  error: unknown
}

const noRowState: RowState = { errors: {}, failure: null }

/** The field whose error a change to each part of the draft may have fixed. */
const fieldOfDraftKey: Record<keyof TransactionDraft, TransactionField> = {
  amount: 'amount',
  kind: 'category',
  categoryId: 'category',
  date: 'date',
  merchant: 'merchant',
  note: 'note',
}

function fieldsOf(patch: Partial<TransactionDraft>): TransactionField[] {
  return (Object.keys(patch) as (keyof TransactionDraft)[]).map((key) => fieldOfDraftKey[key])
}

function headerId(id: string): string {
  return `entry-${id}`
}

/**
 * The confirm sheet of quick entry (ADR-0076): every proposal as a compact row that opens in
 * place into the manual form's fields. "Save all" sends one POST /api/transactions per item, in
 * the order typed; saved ones leave the sheet, a failed one stays with its error. `onClose`
 * gets how many were saved and how many are left once the sheet should close.
 */
export function ConfirmSheet({
  session,
  open,
  onClose,
}: {
  session: EntrySession
  open: boolean
  onClose: (result: { saved: number; left: number }) => void
}) {
  const queryClient = useQueryClient()
  const categoriesResult = useQuery(categoriesQuery)
  const categories = categoriesResult.data?.categories ?? []
  const today = todayIn()
  const [proposals, setProposals] = useState(session.proposals)
  const [saved, setSaved] = useState(0)
  const [rows, setRows] = useState<ReadonlyMap<string, RowState>>(() => new Map())
  const [openId, setOpenId] = useState<string | null>(null)
  const [choosing, setChoosing] = useState(false)
  const [focusRequest, setFocusRequest] = useState<{ id: string; n: number } | null>(null)
  // Focus goes to the sheet itself, not a field, so the phone's keyboard stays down.
  const popupRef = useRef<HTMLDivElement>(null)
  const choiceRef = useRef<HTMLButtonElement>(null)
  const questionId = useId()

  const save = useMutation({
    // One at a time, in the order typed, so a merchant rule ends with the last one (ADR-0076).
    mutationFn: async (batch: { id: string; req: CreateTransactionRequest }[]) => {
      const results: SaveResult[] = []
      for (const { id, req } of batch) {
        try {
          await createTransaction(req)
          results.push({ id, error: null })
        } catch (error) {
          results.push({ id, error })
        }
      }
      return results
    },
  })

  useEffect(() => {
    if (focusRequest === null) {
      return
    }
    const header = document.getElementById(headerId(focusRequest.id))
    header?.focus()
    header?.scrollIntoView({ block: 'start' })
  }, [focusRequest])

  const live = splitProposals(proposals, categories, today)
  const showChoice = choosing && live.ready.length > 0 && live.incomplete.length > 0

  useEffect(() => {
    if (showChoice) {
      choiceRef.current?.focus()
    }
  }, [showChoice])

  function focusRow(id: string) {
    setFocusRequest((r) => ({ id, n: (r?.n ?? 0) + 1 }))
  }

  function reveal(id: string) {
    setOpenId(id)
    focusRow(id)
  }

  function close() {
    if (save.isPending) {
      return
    }
    onClose({ saved, left: proposals.length })
  }

  function change(id: string, patch: Partial<TransactionDraft>) {
    setProposals((ps) =>
      ps.map((p) => (p.id === id ? { ...p, draft: { ...p.draft, ...patch }, unsure: false } : p)),
    )
    setRows((rs) => {
      const state = rs.get(id)
      if (!state) {
        return rs
      }
      const fixed = fieldsOf(patch)
      const errors: FieldErrors = Object.fromEntries(
        Object.entries(state.errors).filter(([field]) => !(fixed as string[]).includes(field)),
      )
      return new Map(rs).set(id, { ...state, errors })
    })
  }

  function remove(id: string) {
    const index = proposals.findIndex((p) => p.id === id)
    const next = proposals.filter((p) => p.id !== id)
    if (next.length === 0) {
      onClose({ saved, left: 0 })
      return
    }
    setProposals(next)
    if (openId === id) {
      setOpenId(null)
    }
    // The Remove button is gone; keep the focus in the list.
    const neighbour = next.at(Math.min(index, next.length - 1))
    if (neighbour) {
      focusRow(neighbour.id)
    }
  }

  /** Marks the incomplete ones with their errors and opens the first. */
  function markIncomplete(incomplete: ProposalSplit['incomplete']) {
    setRows((rs) => {
      const next = new Map(rs)
      for (const { proposal, errors } of incomplete) {
        next.set(proposal.id, { errors, failure: null })
      }
      return next
    })
    const first = incomplete.at(0)
    if (first) {
      reveal(first.proposal.id)
    }
  }

  function send(ready: ProposalSplit['ready']) {
    if (ready.length === 0) {
      return
    }
    const before = proposals.length
    setRows((rs) => {
      const next = new Map(rs)
      for (const { proposal } of ready) {
        next.delete(proposal.id)
      }
      return next
    })
    save.mutate(
      ready.map(({ proposal, value }) => ({
        id: proposal.id,
        req: textTransactionRequest(proposal, value),
      })),
      {
        onSuccess: (results) => {
          finish(results, before)
        },
      },
    )
  }

  function finish(results: SaveResult[], before: number) {
    const savedIds = new Set(results.filter((r) => r.error === null).map((r) => r.id))
    if (savedIds.size > 0) {
      void queryClient.invalidateQueries({ queryKey: transactionsKey })
    }
    const total = saved + savedIds.size
    // Remove is off while saving, so only saves have changed the list.
    const left = before - savedIds.size
    if (left === 0) {
      onClose({ saved: total, left: 0 })
      return
    }
    const failed = new Map<string, RowState>()
    let firstFieldError: string | null = null
    for (const { id, error } of results) {
      if (error === null) {
        continue
      }
      const field = isApiError(error, 'invalid_input') ? fieldOfApiError(error.message) : null
      if (field) {
        failed.set(id, { errors: { [field]: apiFieldMessages[field] }, failure: null })
        firstFieldError ??= id
        if (field === 'category') {
          // The category may have been archived elsewhere; show the current list.
          void queryClient.invalidateQueries({ queryKey: categoriesQuery.queryKey })
        }
      } else {
        failed.set(id, { errors: {}, failure: errorText(error) })
      }
    }
    setProposals((ps) => ps.filter((p) => !savedIds.has(p.id)))
    setSaved(total)
    setRows((rs) => new Map([...rs, ...failed]))
    if (firstFieldError !== null) {
      reveal(firstFieldError)
    }
  }

  function saveAll() {
    if (save.isPending) {
      return
    }
    if (live.incomplete.length === 0) {
      send(live.ready)
    } else if (live.ready.length === 0) {
      markIncomplete(live.incomplete)
    } else {
      setChoosing(true)
    }
  }

  const aiNotice = t.aiNotice(session.ai)
  const categoryById = new Map(categories.map((c) => [c.id, c]))

  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          close()
        }
      }}
    >
      <SheetContent ref={popupRef} initialFocus={popupRef}>
        <div className="flex flex-col gap-4">
          <SheetHeader>
            <SheetTitle>{t.sheetTitle}</SheetTitle>
            <SheetDescription>{t.sheetDescription(proposals.length)}</SheetDescription>
          </SheetHeader>
          {aiNotice && (
            <p className="flex gap-3 rounded-xl bg-muted px-4 py-3">
              <InfoIcon aria-hidden="true" className="mt-0.5 size-5 shrink-0" />
              <span>{aiNotice}</span>
            </p>
          )}
          <ul className="flex flex-col divide-y divide-border border-t border-border">
            {proposals.map((proposal) => (
              <ProposalRow
                key={proposal.id}
                proposal={proposal}
                category={
                  proposal.draft.categoryId === null
                    ? undefined
                    : categoryById.get(proposal.draft.categoryId)
                }
                incomplete={checkProposal(proposal, categories, today)}
                state={rows.get(proposal.id) ?? noRowState}
                open={openId === proposal.id}
                busy={save.isPending}
                categories={categories}
                today={today}
                onToggle={() => {
                  setOpenId((current) => (current === proposal.id ? null : proposal.id))
                }}
                onChange={(patch) => {
                  change(proposal.id, patch)
                }}
                onRemove={() => {
                  remove(proposal.id)
                }}
                onDone={() => {
                  setOpenId(null)
                  focusRow(proposal.id)
                }}
              />
            ))}
          </ul>
          {/* Within thumb reach while the list scrolls under it. */}
          <div className="sticky -bottom-4 z-10 -mx-4 border-t border-border bg-popover px-4 pt-4 pb-4 md:static md:mx-0 md:px-0 md:pb-0">
            {showChoice ? (
              <div role="group" aria-labelledby={questionId} className="flex flex-col gap-3">
                <p id={questionId} className="font-semibold">
                  {t.someIncomplete(live.incomplete.length)}
                </p>
                <Button
                  ref={choiceRef}
                  size="lg"
                  onClick={() => {
                    setChoosing(false)
                    send(live.ready)
                    markIncomplete(live.incomplete)
                  }}
                >
                  {t.saveReady(live.ready.length)}
                </Button>
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => {
                    setChoosing(false)
                    markIncomplete(live.incomplete)
                  }}
                >
                  {t.finishFirst}
                </Button>
              </div>
            ) : (
              <div className="flex gap-3 md:justify-end">
                <Button
                  variant="outline"
                  size="lg"
                  disabled={save.isPending}
                  focusableWhenDisabled
                  onClick={close}
                >
                  {common.cancel}
                </Button>
                <Button
                  size="lg"
                  className="flex-1 md:flex-none"
                  disabled={save.isPending}
                  focusableWhenDisabled
                  onClick={saveAll}
                >
                  {save.isPending ? t.saving : t.saveAll}
                </Button>
              </div>
            )}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

function ProposalRow({
  proposal,
  category,
  incomplete,
  state,
  open,
  busy,
  categories,
  today,
  onToggle,
  onChange,
  onRemove,
  onDone,
}: {
  proposal: Proposal
  /** The chosen category, when it is one of the user's. */
  category: Category | undefined
  /** The rules the row still breaks, as the manual form would say them. */
  incomplete: ReturnType<typeof checkProposal>
  state: RowState
  open: boolean
  busy: boolean
  categories: readonly Category[]
  today: string
  onToggle: () => void
  onChange: (patch: Partial<TransactionDraft>) => void
  onRemove: () => void
  onDone: () => void
}) {
  const panelId = useId()
  const { draft } = proposal
  const title = proposalTitle(proposal, category)
  const showsCategory = category !== undefined && title !== category.name
  const amount = parseAmountInput(draft.amount)
  const variant = (category?.kind ?? draft.kind) === 'income' ? 'income' : 'expense'
  const todo = incomplete.ok ? [] : Object.values(incomplete.errors)

  return (
    <li className="flex flex-col">
      <div className="flex items-start gap-1">
        <button
          id={headerId(proposal.id)}
          type="button"
          aria-expanded={open}
          aria-controls={open ? panelId : undefined}
          onClick={onToggle}
          className="flex min-h-14 min-w-0 flex-1 items-start gap-3 rounded-lg py-3 text-left select-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring active:bg-muted"
        >
          <CategoryIcon icon={category?.icon ?? ''} />
          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
            {/* One line, as in History: a clamped second line would let Thai marks stacked above
                the hidden third line show through. The whole text is in Description when open. */}
            <span className="truncate">{title}</span>
            <span className="text-sm [overflow-wrap:anywhere] text-muted-foreground">
              {showsCategory && `${category.name} · `}
              <span className="whitespace-nowrap">
                {isDate(draft.date) ? dayLabel(draft.date) : draft.date}
              </span>
            </span>
            {(proposal.unsure || todo.length > 0) && (
              <span className="flex flex-wrap items-center gap-x-3 gap-y-1 pt-1 text-sm">
                {proposal.unsure && (
                  <span className="rounded-full bg-muted px-2 py-0.5 font-semibold">
                    {t.checkThis}
                  </span>
                )}
                {todo.length > 0 && (
                  <span className="flex items-start gap-1">
                    <CircleAlertIcon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
                    <span>{todo.join(' ')}</span>
                  </span>
                )}
              </span>
            )}
          </span>
          {amount.ok ? (
            <Amount value={amount.value} variant={variant} />
          ) : (
            draft.amount.trim() !== '' && (
              <span className="max-w-20 truncate font-semibold">{draft.amount}</span>
            )
          )}
        </button>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t.remove(title)}
          disabled={busy}
          focusableWhenDisabled
          onClick={onRemove}
          // The 44px target reaches into the sheet's margin, leaving the text more room.
          className="mt-1.5 -mr-3 text-muted-foreground"
        >
          <XIcon />
        </Button>
      </div>
      {state.failure && <FormAlert className="mb-3">{state.failure}</FormAlert>}
      {open && (
        <div id={panelId} className="flex flex-col gap-6 pt-1 pb-4">
          <TransactionFields
            draft={draft}
            onChange={onChange}
            errors={state.errors}
            categories={{ status: 'success', categories }}
            today={today}
          />
          <Button variant="outline" size="lg" onClick={onDone}>
            {t.done}
          </Button>
        </div>
      )}
    </li>
  )
}
