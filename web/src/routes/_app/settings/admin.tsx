import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { cn } from 'cn'
import { useCallback, useEffect, useState } from 'react'
import type { SubmitEvent } from 'react'

import { inviteUser, listUsers, removeUser, sendSetPasswordLink } from '@/api/admin'
import { ApiError } from '@/api/client'
import type { InviteResponse, UserItem } from '@/api/types'
import { BackLink, PageTitle } from '@/components/layout'
import { FormAlert, Notice } from '@/components/messages'
import { TextField } from '@/components/text-field'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { errorText, isApiError } from '@/lib/errors'
import { formatDate, formatDateTime } from '@/lib/format'
import { admin as t } from '@/messages/admin'
import { common } from '@/messages/common'
import { settings } from '@/messages/settings'
import { meQuery } from '@/session/session'

// Operators only (ADR-0019, ADR-0068). Anyone else is sent back to Settings, both by this
// guard and when the API answers 403. The list never holds financial data.
export const Route = createFileRoute('/_app/settings/admin')({
  beforeLoad: async ({ context }) => {
    let role: string
    try {
      role = (await context.queryClient.query({ ...meQuery, staleTime: 'static' })).role
    } catch (err) {
      if (err instanceof ApiError && err.code === 'unauthenticated') {
        // The client has already ended the session; go to Login.
        throw redirect({ to: '/login', replace: true })
      }
      throw err
    }
    if (role !== 'operator') {
      throw redirect({ to: '/settings', replace: true })
    }
  },
  component: AdminPage,
})

const usersQueryKey = ['admin', 'users'] as const

/** Sends a non-operator back to Settings when the API says 403, and rechecks the role. */
function useForbiddenGuard(): (err: unknown) => void {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  return useCallback(
    (err: unknown) => {
      if (isApiError(err, 'forbidden')) {
        void queryClient.invalidateQueries({ queryKey: meQuery.queryKey })
        void navigate({ to: '/settings', replace: true })
      }
    },
    [queryClient, navigate],
  )
}

function AdminPage() {
  const queryClient = useQueryClient()
  const onForbidden = useForbiddenGuard()
  const me = useQuery(meQuery)
  const users = useQuery({
    queryKey: usersQueryKey,
    queryFn: ({ signal }) => listUsers(signal),
  })
  const [notice, setNotice] = useState<string | null>(null)
  const [inviteOpen, setInviteOpen] = useState(false)
  // The target stays set while the sheet closes, so its text does not vanish mid-animation.
  const [removeTarget, setRemoveTarget] = useState<UserItem | null>(null)
  const [removeOpen, setRemoveOpen] = useState(false)
  const [sheetKey, setSheetKey] = useState(0)

  useEffect(() => {
    if (users.isError) {
      onForbidden(users.error)
    }
  }, [users.isError, users.error, onForbidden])

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: usersQueryKey })
  }

  return (
    <>
      <div className="flex flex-col gap-2">
        <BackLink to="/settings" label={settings.title} />
        <PageTitle>{t.title}</PageTitle>
        <p className="text-muted-foreground">{t.intro}</p>
      </div>
      <Notice>{notice}</Notice>

      {users.isPending && <UserListSkeleton />}
      {users.isError && !isApiError(users.error, 'forbidden') && (
        <div className="flex flex-col gap-4">
          <FormAlert>
            <p>{t.loadError}</p>
            <p>{errorText(users.error)}</p>
          </FormAlert>
          <Button
            variant="outline"
            onClick={() => {
              void users.refetch()
            }}
          >
            {common.tryAgain}
          </Button>
        </div>
      )}
      {users.isSuccess &&
        (users.data.users.length === 0 ? (
          <p className="border-y border-border py-8 text-muted-foreground">{t.empty}</p>
        ) : (
          <ul
            aria-label={t.title}
            className="flex flex-col divide-y divide-border border-y border-border"
          >
            {users.data.users.map((user) => (
              <UserRow
                key={user.id}
                user={user}
                isSelf={me.data?.id === user.id}
                onRemove={() => {
                  setNotice(null)
                  setSheetKey((k) => k + 1)
                  setRemoveTarget(user)
                  setRemoveOpen(true)
                }}
                onError={onForbidden}
                onChanged={refresh}
              />
            ))}
          </ul>
        ))}

      {/* The main action stays within thumb reach, just above the app bar on a phone. */}
      <div className="sticky bottom-[calc(5rem+env(safe-area-inset-bottom))] mt-auto md:static">
        <Button
          size="lg"
          className="w-full shadow-[0_4px_16px_rgb(31_122_84/0.3)] md:shadow-none"
          onClick={() => {
            setNotice(null)
            setSheetKey((k) => k + 1)
            setInviteOpen(true)
          }}
        >
          {t.invite}
        </Button>
      </div>

      <Sheet key={`invite-${String(sheetKey)}`} open={inviteOpen} onOpenChange={setInviteOpen}>
        <SheetContent>
          <InviteForm
            onDone={(text) => {
              setInviteOpen(false)
              setNotice(text)
            }}
            onError={onForbidden}
            onChanged={refresh}
          />
        </SheetContent>
      </Sheet>

      <Sheet key={`remove-${String(sheetKey)}`} open={removeOpen} onOpenChange={setRemoveOpen}>
        <SheetContent>
          {removeTarget && (
            <RemoveForm
              user={removeTarget}
              onRemoved={() => {
                setNotice(t.removed(removeTarget.email))
                setRemoveOpen(false)
              }}
              onError={onForbidden}
              onChanged={refresh}
            />
          )}
        </SheetContent>
      </Sheet>
    </>
  )
}

function UserListSkeleton() {
  return (
    <ul
      aria-hidden="true"
      data-testid="users-skeleton"
      className="flex flex-col divide-y divide-border border-y border-border"
    >
      {[0, 1, 2].map((i) => (
        <li key={i} className="flex flex-col gap-2 py-4">
          <Skeleton className="h-6 w-3/4" />
          <Skeleton className="h-5 w-1/2" />
          <Skeleton className="h-11 w-40 rounded-xl" />
        </li>
      ))}
    </ul>
  )
}

function StatusMark({ status }: { status: UserItem['status'] }) {
  return (
    <span
      className={cn(
        'shrink-0 rounded-full px-3 py-1 text-sm',
        status === 'active' ? 'bg-primary/8 text-primary' : 'bg-muted text-muted-foreground',
      )}
    >
      {status === 'active' ? t.active : t.invited}
    </span>
  )
}

interface RowResult {
  ok: boolean
  text: string
}

function UserRow({
  user,
  isSelf,
  onRemove,
  onError,
  onChanged,
}: {
  user: UserItem
  isSelf: boolean
  onRemove: () => void
  onError: (err: unknown) => void
  onChanged: () => void
}) {
  const [result, setResult] = useState<RowResult | null>(null)
  const send = useMutation({
    mutationFn: () => sendSetPasswordLink(user.id),
    onMutate: () => {
      setResult(null)
    },
    onSuccess: ({ email_sent }) => {
      setResult({
        ok: email_sent,
        text: email_sent ? t.linkSent(user.email) : t.linkNotSent(user.email),
      })
      onChanged()
    },
    onError: (err) => {
      onError(err)
      setResult({ ok: false, text: errorText(err) })
    },
  })

  return (
    <li className="flex flex-col gap-3 py-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col">
          <p className="font-semibold [overflow-wrap:anywhere]">
            {user.email}
            {isSelf && <span className="font-normal text-muted-foreground"> ({t.you})</span>}
          </p>
          {user.display_name !== '' && (
            <p className="[overflow-wrap:anywhere] text-muted-foreground">{user.display_name}</p>
          )}
        </div>
        <StatusMark status={user.status} />
      </div>
      <p className="flex flex-col text-sm text-muted-foreground">
        <span>{t.joined(formatDate(user.created_at))}</span>
        <span>
          {user.last_active_at === null
            ? t.neverActive
            : t.lastActive(formatDateTime(user.last_active_at))}
        </span>
      </p>
      <div className="flex flex-wrap gap-3">
        <Button
          variant="outline"
          size="sm"
          disabled={send.isPending}
          focusableWhenDisabled
          onClick={() => {
            send.mutate()
          }}
        >
          {send.isPending ? t.sendingLink : t.sendLink}
        </Button>
        {/* The API refuses an operator's own removal; they use Delete account in Settings. */}
        {!isSelf && (
          <Button variant="destructive-ghost" size="sm" onClick={onRemove}>
            {t.remove}
          </Button>
        )}
      </div>
      {result &&
        (result.ok ? <Notice>{result.text}</Notice> : <FormAlert>{result.text}</FormAlert>)}
    </li>
  )
}

function InviteForm({
  onDone,
  onError,
  onChanged,
}: {
  onDone: (notice: string) => void
  onError: (err: unknown) => void
  onChanged: () => void
}) {
  const [email, setEmail] = useState('')
  const [fieldError, setFieldError] = useState<string | null>(null)
  const [created, setCreated] = useState<InviteResponse | null>(null)

  const invite = useMutation({
    mutationFn: inviteUser,
    onSuccess: (res) => {
      onChanged()
      if (res.email_sent) {
        onDone(t.invitedOk(res.user.email))
      } else {
        setCreated(res)
      }
    },
    onError: (err) => {
      onError(err)
      // conflict: the email already has an account; invalid_input: not one plain address.
      if (isApiError(err, 'conflict') || isApiError(err, 'invalid_input')) {
        setFieldError(errorText(err))
      }
    },
  })

  if (created) {
    return <InviteNotSent user={created.user} onDone={onDone} onError={onError} />
  }

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (invite.isPending) {
      return
    }
    const trimmed = email.trim()
    const err = trimmed === '' ? t.enterEmail : null
    setFieldError(err)
    if (err) {
      return
    }
    invite.mutate({ email: trimmed })
  }

  const formError =
    invite.isError &&
    !isApiError(invite.error, 'conflict') &&
    !isApiError(invite.error, 'invalid_input')
      ? errorText(invite.error)
      : null

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.inviteTitle}</SheetTitle>
        <SheetDescription>{t.inviteIntro}</SheetDescription>
      </SheetHeader>
      <TextField
        label={t.inviteEmail}
        name="invite-email"
        type="email"
        inputMode="email"
        autoComplete="off"
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        value={email}
        error={fieldError}
        onChange={(e) => {
          setEmail(e.target.value)
        }}
      />
      {formError && <FormAlert>{formError}</FormAlert>}
      <SheetFooter className="mt-0">
        <Button type="submit" size="lg" disabled={invite.isPending} focusableWhenDisabled>
          {invite.isPending ? t.inviting : t.inviteSubmit}
        </Button>
      </SheetFooter>
    </form>
  )
}

/** The account exists but the email failed (ADR-0068): say so, and offer a new link. */
function InviteNotSent({
  user,
  onDone,
  onError,
}: {
  user: UserItem
  onDone: (notice: string) => void
  onError: (err: unknown) => void
}) {
  const [failure, setFailure] = useState<string | null>(null)
  const send = useMutation({
    mutationFn: () => sendSetPasswordLink(user.id),
    onMutate: () => {
      setFailure(null)
    },
    onSuccess: ({ email_sent }) => {
      if (email_sent) {
        onDone(t.linkSent(user.email))
      } else {
        setFailure(t.linkNotSent(user.email))
      }
    },
    onError: (err) => {
      onError(err)
      setFailure(errorText(err))
    },
  })

  return (
    <div className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.inviteTitle}</SheetTitle>
        <SheetDescription>{t.inviteNotSent(user.email)}</SheetDescription>
      </SheetHeader>
      <p role="alert">{t.inviteNotSentHint}</p>
      {failure && <FormAlert>{failure}</FormAlert>}
      <SheetFooter className="mt-0">
        <Button
          size="lg"
          disabled={send.isPending}
          focusableWhenDisabled
          onClick={() => {
            send.mutate()
          }}
        >
          {send.isPending ? t.sendingLink : t.sendLink}
        </Button>
      </SheetFooter>
    </div>
  )
}

function RemoveForm({
  user,
  onRemoved,
  onError,
  onChanged,
}: {
  user: UserItem
  onRemoved: () => void
  onError: (err: unknown) => void
  onChanged: () => void
}) {
  const remove = useMutation({
    mutationFn: () => removeUser(user.id),
    onSuccess: () => {
      onChanged()
      onRemoved()
    },
    onError,
  })

  return (
    <div className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle className="[overflow-wrap:anywhere]">{t.removeTitle(user.email)}</SheetTitle>
        <SheetDescription>{t.removeWarning}</SheetDescription>
      </SheetHeader>
      {remove.isError && <FormAlert>{errorText(remove.error)}</FormAlert>}
      <SheetFooter className="mt-0">
        <SheetClose render={<Button variant="outline" size="lg" />}>{common.cancel}</SheetClose>
        <Button
          variant="destructive"
          size="lg"
          disabled={remove.isPending}
          focusableWhenDisabled
          onClick={() => {
            remove.mutate()
          }}
        >
          {remove.isPending ? t.removing : t.removeSubmit}
        </Button>
      </SheetFooter>
    </div>
  )
}
