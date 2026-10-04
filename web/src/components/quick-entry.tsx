import { useMutation, usePrefetchQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { SubmitEvent } from 'react'

import { parseEntry } from '@/api/entry'
import { ConfirmSheet } from '@/components/confirm-sheet'
import type { EntrySession } from '@/components/confirm-sheet'
import { ActionBar, FormSheet } from '@/components/layout'
import { FormAlert } from '@/components/messages'
import { TextAreaField } from '@/components/text-field'
import { Button } from '@/components/ui/button'
import { today as todayIn } from '@/lib/dates'
import { errorText, isApiError } from '@/lib/errors'
import { checkEntryText, entryTextErrorOfApi, savedNotice, toProposal } from '@/lib/quick-entry'
import { entry as t } from '@/messages/entry'
import { categoriesQuery } from '@/queries/money'

/**
 * Quick entry on the Add screen (ADR-0076): a typed text, read by POST /api/entry/parse, then
 * confirmed in a sheet before anything is saved. `onNotice` says how many were saved.
 */
export function QuickEntry({ onNotice }: { onNotice: (text: string | null) => void }) {
  const queryClient = useQueryClient()
  // The proposals are matched against the categories, so load them while the user types.
  usePrefetchQuery(categoriesQuery)
  const [text, setText] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [session, setSession] = useState<EntrySession | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)

  const parse = useMutation({
    mutationFn: async (typed: string) => {
      const [answer, list] = await Promise.all([
        parseEntry(typed),
        // Cached categories are used as they are; they are only loaded when there are none.
        queryClient.query({ ...categoriesQuery, staleTime: 'static' }),
      ])
      return { answer, categories: list.categories }
    },
    onSuccess: ({ answer, categories }) => {
      if (answer.items.length === 0) {
        setError(t.noItem)
        return
      }
      const today = todayIn()
      // Each proposal gets its transaction id here, once (ADR-0040).
      setSession((s) => ({
        key: (s?.key ?? 0) + 1,
        ai: answer.ai,
        proposals: answer.items.map((item) => toProposal(item, categories, today)),
      }))
      setSheetOpen(true)
    },
    onError: (err) => {
      if (isApiError(err, 'invalid_input')) {
        setError(entryTextErrorOfApi(err.message))
      }
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (parse.isPending) {
      return
    }
    onNotice(null)
    const checked = checkEntryText(text)
    if (!checked.ok) {
      setError(checked.error)
      return
    }
    setError(null)
    parse.mutate(checked.text)
  }

  const formError =
    parse.isError &&
    !(isApiError(parse.error, 'invalid_input') && entryTextErrorOfApi(parse.error.message) !== null)
      ? errorText(parse.error)
      : null

  return (
    <>
      <form noValidate onSubmit={submit} className="flex flex-1 flex-col gap-6">
        <FormSheet>
          <TextAreaField
            label={t.textLabel}
            hint={t.textHint}
            name="entries"
            rows={4}
            autoComplete="off"
            // Descriptions are kept as typed, so the keyboard should not capitalise them.
            autoCapitalize="none"
            // A line break separates entries, so the Enter key keeps its meaning.
            enterKeyHint="enter"
            value={text}
            error={error}
            onChange={(e) => {
              setText(e.target.value)
            }}
          />
        </FormSheet>
        {formError && <FormAlert>{formError}</FormAlert>}
        <ActionBar>
          <Button
            type="submit"
            size="lg"
            className="w-full shadow-[0_4px_16px_rgb(31_122_84/0.3)] md:shadow-none"
            disabled={parse.isPending}
            focusableWhenDisabled
          >
            {parse.isPending ? t.reading : t.read}
          </Button>
        </ActionBar>
      </form>
      {/* Outside the form: the sheet is portalled, but React events still bubble through it. */}
      {session && (
        <ConfirmSheet
          key={session.key}
          session={session}
          open={sheetOpen}
          onClose={({ saved, left, rest }) => {
            setSheetOpen(false)
            if (saved > 0) {
              // Only what is still to save stays in the box, one entry a line, so reading it
              // again cannot propose a saved entry twice (ADR-0083); empty when all is handled.
              setText(rest.join('\n'))
            }
            onNotice(savedNotice(saved, left))
          }}
        />
      )}
    </>
  )
}
