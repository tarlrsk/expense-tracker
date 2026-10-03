import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect, useRouter } from '@tanstack/react-router'
import { useState } from 'react'
import type { SubmitEvent } from 'react'

import { login } from '@/api/auth'
import { FormSheet, PublicLayout } from '@/components/layout'
import { FormAlert } from '@/components/messages'
import { TextField } from '@/components/text-field'
import { Button } from '@/components/ui/button'
import { errorText } from '@/lib/errors'
import { common } from '@/messages/common'
import { login as t } from '@/messages/login'
import { safeRedirect } from '@/session/redirect'
import { isLoggedIn, startSession } from '@/session/session'

interface LoginSearch {
  /** Where to go after logging in; only in-app paths are followed (safeRedirect). */
  redirect?: string
}

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>): LoginSearch =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  beforeLoad: ({ search }) => {
    if (isLoggedIn()) {
      throw redirect({ href: safeRedirect(search.redirect), replace: true })
    }
  },
  component: LoginPage,
})

function LoginPage() {
  const search = Route.useSearch()
  const router = useRouter()
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [emailError, setEmailError] = useState<string | null>(null)
  const [passwordError, setPasswordError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: login,
    onSuccess: (session) => {
      startSession(queryClient, session.token)
      void router.navigate({ href: safeRedirect(search.redirect), replace: true })
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mutation.isPending) {
      return
    }
    const trimmed = email.trim()
    const eErr = trimmed === '' ? t.enterEmail : null
    const pErr = password === '' ? common.enterPassword : null
    setEmailError(eErr)
    setPasswordError(pErr)
    if (eErr || pErr) {
      return
    }
    mutation.mutate({ email: trimmed, password })
  }

  return (
    <PublicLayout>
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-2">
          <h1 className="text-display font-semibold">{t.title}</h1>
          <p className="text-muted-foreground">{t.intro}</p>
        </div>
        <FormSheet>
          <form noValidate onSubmit={submit} className="flex flex-col gap-6">
            <TextField
              label={t.email}
              name="email"
              type="email"
              inputMode="email"
              autoComplete="username"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              value={email}
              error={emailError}
              onChange={(e) => {
                setEmail(e.target.value)
              }}
            />
            <TextField
              label={t.password}
              name="password"
              type="password"
              autoComplete="current-password"
              revealable
              value={password}
              error={passwordError}
              onChange={(e) => {
                setPassword(e.target.value)
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
        <p className="text-sm text-muted-foreground">{t.noAccount}</p>
      </div>
    </PublicLayout>
  )
}
