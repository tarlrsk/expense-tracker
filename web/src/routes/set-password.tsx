import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, createFileRoute, useRouter } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import type { SubmitEvent } from 'react'

import { setPassword } from '@/api/auth'
import { FormSheet, PublicLayout } from '@/components/layout'
import { FormAlert } from '@/components/messages'
import { TextField } from '@/components/text-field'
import { Button } from '@/components/ui/button'
import { errorText, isApiError } from '@/lib/errors'
import { passwordRuleError } from '@/lib/password'
import { common } from '@/messages/common'
import { setPassword as t } from '@/messages/set-password'
import { readFragmentToken, removeFragment } from '@/session/fragment'
import { homePath } from '@/session/redirect'
import { startSession } from '@/session/session'

// Opened from the emailed link `/set-password#token=…` (ADR-0068). Public: it works whether or
// not someone is logged in on this device; a success replaces any session with the new one.
export const Route = createFileRoute('/set-password')({
  component: SetPasswordPage,
})

function SetPasswordPage() {
  // Read once, kept only in memory; sent nowhere but the API call below.
  const [token] = useState(() => readFragmentToken(window.location.hash))
  useEffect(() => {
    removeFragment()
  }, [])

  return (
    <PublicLayout>
      {token ? <SetPasswordForm token={token} /> : <BadLink message={t.noLink} />}
    </PublicLayout>
  )
}

function SetPasswordForm({ token }: { token: string }) {
  const router = useRouter()
  const queryClient = useQueryClient()
  const [password, setPasswordValue] = useState('')
  const [fieldError, setFieldError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: setPassword,
    onSuccess: (session) => {
      startSession(queryClient, session.token)
      void router.navigate({ to: homePath, replace: true })
    },
  })

  // The password rule is checked here first, so an `invalid_input` answer means the link.
  if (isApiError(mutation.error, 'invalid_input')) {
    return <BadLink message={errorText(mutation.error)} />
  }

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mutation.isPending) {
      return
    }
    const err = password === '' ? common.passwordTooShort : passwordRuleError(password)
    setFieldError(err)
    if (err) {
      return
    }
    mutation.mutate({ token, password })
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <h1 className="text-display font-semibold">{t.title}</h1>
        <p className="text-muted-foreground">{t.intro}</p>
      </div>
      <FormSheet>
        <form noValidate onSubmit={submit} className="flex flex-col gap-6">
          <TextField
            label={t.password}
            name="new-password"
            type="password"
            autoComplete="new-password"
            hint={common.passwordRule}
            revealable
            value={password}
            error={fieldError}
            onChange={(e) => {
              setPasswordValue(e.target.value)
            }}
          />
          {mutation.isError && <FormAlert>{errorText(mutation.error)}</FormAlert>}
          <Button
            type="submit"
            size="lg"
            disabled={mutation.isPending}
            focusableWhenDisabled
            className="w-full"
          >
            {mutation.isPending ? t.pending : t.submit}
          </Button>
        </form>
      </FormSheet>
    </div>
  )
}

function BadLink({ message }: { message: string }) {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-display font-semibold">{t.noLinkTitle}</h1>
      <FormAlert>
        <p>{message}</p>
        <p>{t.askNewLink}</p>
      </FormAlert>
      <Link
        to="/login"
        className="flex h-12 items-center justify-center rounded-xl border border-input bg-card px-5 text-base font-semibold select-none"
      >
        {t.toLogin}
      </Link>
    </div>
  )
}
