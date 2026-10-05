import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import {
  CheckIcon,
  ChevronRightIcon,
  KeyRoundIcon,
  SparklesIcon,
  TagsIcon,
  UsersIcon,
} from 'lucide-react'
import { useId, useState } from 'react'
import type { SubmitEvent } from 'react'

import { changePassword, deleteMe, updateMe } from '@/api/me'
import { descriptionStyles } from '@/api/types'
import type { DescriptionStyle, Profile } from '@/api/types'
import { FormSheet, PageTitle } from '@/components/layout'
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
import { characterCount, passwordRuleError } from '@/lib/password'
import { common } from '@/messages/common'
import { settings as t } from '@/messages/settings'
import { endSession, logOut, meQuery } from '@/session/session'

export const Route = createFileRoute('/_app/settings/')({
  component: SettingsPage,
})

const maxDisplayNameLength = 50

function SettingsPage() {
  const me = useQuery(meQuery)
  const [notice, setNotice] = useState<string | null>(null)

  return (
    <>
      <PageTitle>{t.title}</PageTitle>
      <Notice>{notice}</Notice>
      {me.isPending && <SettingsSkeleton />}
      {me.isError && (
        <div className="flex flex-col gap-4">
          <FormAlert>
            <p>{t.loadError}</p>
            <p>{errorText(me.error)}</p>
          </FormAlert>
          <Button
            variant="outline"
            onClick={() => {
              void me.refetch()
            }}
          >
            {common.tryAgain}
          </Button>
        </div>
      )}
      {me.isSuccess && <SettingsContent profile={me.data} onNotice={setNotice} />}
    </>
  )
}

function SettingsSkeleton() {
  return (
    <div className="flex flex-col gap-6" data-testid="settings-skeleton">
      <FormSheet>
        <div className="flex flex-col gap-2">
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-6 w-56" />
        </div>
        <div className="flex flex-col gap-2">
          <Skeleton className="h-5 w-28" />
          <Skeleton className="h-11 w-full rounded-xl" />
        </div>
      </FormSheet>
      <Skeleton className="h-14 w-full" />
    </div>
  )
}

function SettingsContent({
  profile,
  onNotice,
}: {
  profile: Profile
  onNotice: (text: string | null) => void
}) {
  const [changeOpen, setChangeOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [styleOpen, setStyleOpen] = useState(false)
  // A new key for each opening, so a sheet's form always starts empty.
  const [sheetKey, setSheetKey] = useState(0)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [loggingOut, setLoggingOut] = useState(false)

  async function handleLogOut() {
    setLoggingOut(true)
    onNotice(null)
    await logOut(queryClient)
    await navigate({ to: '/login', replace: true })
  }

  return (
    <>
      <DisplayNameForm key={profile.id} profile={profile} onNotice={onNotice} />

      <ul className="flex flex-col divide-y divide-border border-y border-border">
        <li>
          <Link
            to="/settings/categories"
            className="flex min-h-14 w-full items-center gap-3 py-3 text-base select-none"
          >
            <TagsIcon aria-hidden="true" className="size-5 text-muted-foreground" />
            <span className="flex flex-1 flex-col">
              <span>{t.categories}</span>
              <span className="text-sm text-muted-foreground">{t.categoriesHint}</span>
            </span>
            <ChevronRightIcon aria-hidden="true" className="size-5 text-muted-foreground" />
          </Link>
        </li>
        <li>
          <button
            type="button"
            className="flex min-h-14 w-full items-center gap-3 py-3 text-left text-base select-none"
            onClick={() => {
              onNotice(null)
              setSheetKey((k) => k + 1)
              setStyleOpen(true)
            }}
          >
            <SparklesIcon aria-hidden="true" className="size-5 text-muted-foreground" />
            <span className="flex flex-1 flex-col">
              <span>{t.descriptionStyle}</span>
              <span className="text-sm text-muted-foreground">
                {t.descriptionStyles[profile.description_style].label}
              </span>
            </span>
            <ChevronRightIcon aria-hidden="true" className="size-5 text-muted-foreground" />
          </button>
        </li>
        <li>
          <button
            type="button"
            className="flex min-h-14 w-full items-center gap-3 py-3 text-left text-base select-none"
            onClick={() => {
              onNotice(null)
              setSheetKey((k) => k + 1)
              setChangeOpen(true)
            }}
          >
            <KeyRoundIcon aria-hidden="true" className="size-5 text-muted-foreground" />
            <span className="flex-1">{t.changePassword}</span>
            <ChevronRightIcon aria-hidden="true" className="size-5 text-muted-foreground" />
          </button>
        </li>
        {profile.role === 'operator' && (
          <li>
            <Link
              to="/settings/admin"
              className="flex min-h-14 w-full items-center gap-3 py-3 text-base select-none"
            >
              <UsersIcon aria-hidden="true" className="size-5 text-muted-foreground" />
              <span className="flex flex-1 flex-col">
                <span>{t.admin}</span>
                <span className="text-sm text-muted-foreground">{t.adminHint}</span>
              </span>
              <ChevronRightIcon aria-hidden="true" className="size-5 text-muted-foreground" />
            </Link>
          </li>
        )}
      </ul>

      <div className="flex flex-col gap-3">
        <Button
          variant="outline"
          size="lg"
          disabled={loggingOut}
          focusableWhenDisabled
          onClick={() => {
            void handleLogOut()
          }}
        >
          {loggingOut ? t.loggingOut : t.logOut}
        </Button>
        <Button
          variant="destructive-ghost"
          size="lg"
          onClick={() => {
            onNotice(null)
            setSheetKey((k) => k + 1)
            setDeleteOpen(true)
          }}
        >
          {t.deleteAccount}
        </Button>
      </div>

      <DescriptionStyleSheet
        key={`style-${String(sheetKey)}`}
        open={styleOpen}
        onOpenChange={setStyleOpen}
        saved={profile.description_style}
        onSaved={() => {
          setStyleOpen(false)
          onNotice(t.descriptionStyleSaved)
        }}
      />
      <ChangePasswordSheet
        key={`change-${String(sheetKey)}`}
        open={changeOpen}
        onOpenChange={setChangeOpen}
        email={profile.email}
        onChanged={() => {
          setChangeOpen(false)
          onNotice(t.passwordChanged)
        }}
      />
      <DeleteAccountSheet
        key={`delete-${String(sheetKey)}`}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        email={profile.email}
        onDeleted={() => {
          endSession(queryClient)
          void navigate({ to: '/login', replace: true })
        }}
      />
    </>
  )
}

function DisplayNameForm({
  profile,
  onNotice,
}: {
  profile: Profile
  onNotice: (text: string | null) => void
}) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(profile.display_name)
  const [fieldError, setFieldError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: updateMe,
    onSuccess: (updated) => {
      queryClient.setQueryData(meQuery.queryKey, updated)
      setName(updated.display_name)
      onNotice(t.nameSaved)
    },
    onError: (err) => {
      if (isApiError(err, 'invalid_input')) {
        setFieldError(errorText(err))
      }
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mutation.isPending) {
      return
    }
    onNotice(null)
    const err = characterCount(name.trim()) > maxDisplayNameLength ? t.displayNameTooLong : null
    setFieldError(err)
    if (err) {
      return
    }
    mutation.mutate({ display_name: name })
  }

  const formError =
    mutation.isError && !isApiError(mutation.error, 'invalid_input')
      ? errorText(mutation.error)
      : null

  return (
    <FormSheet>
      <div className="flex flex-col gap-1">
        <p className="text-base font-semibold">{t.email}</p>
        <p className="text-base [overflow-wrap:anywhere] text-muted-foreground">{profile.email}</p>
      </div>
      <form noValidate onSubmit={submit} className="flex flex-col gap-4">
        <TextField
          label={t.displayName}
          hint={t.displayNameHint}
          name="display-name"
          autoComplete="nickname"
          value={name}
          error={fieldError}
          onChange={(e) => {
            setName(e.target.value)
          }}
        />
        {formError && <FormAlert>{formError}</FormAlert>}
        <Button
          type="submit"
          variant="secondary"
          disabled={mutation.isPending}
          focusableWhenDisabled
          className="self-start"
        >
          {mutation.isPending ? t.savingName : t.saveName}
        </Button>
      </form>
    </FormSheet>
  )
}

/**
 * Lets a password manager tell which account a password belongs to (a visually hidden
 * username field, as browsers recommend for password forms without one).
 */
function HiddenUsername({ email }: { email: string }) {
  return (
    <input
      type="text"
      name="username"
      autoComplete="username"
      value={email}
      readOnly
      hidden
      tabIndex={-1}
    />
  )
}

/**
 * Choosing the description style (ADR-0081): three native radio options; tapping one saves it at
 * once. Tapping the one already saved just closes the sheet.
 */
function DescriptionStyleSheet({
  open,
  onOpenChange,
  saved,
  onSaved,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  saved: DescriptionStyle
  onSaved: () => void
}) {
  const queryClient = useQueryClient()
  // The option shown as checked: the tapped one while it saves, the saved one otherwise.
  const [selected, setSelected] = useState<DescriptionStyle>(saved)
  const name = useId()

  const mutation = useMutation({
    mutationFn: updateMe,
    onSuccess: (updated) => {
      queryClient.setQueryData(meQuery.queryKey, updated)
      onSaved()
    },
    onError: () => {
      setSelected(saved)
    },
  })

  function choose(style: DescriptionStyle) {
    if (mutation.isPending) {
      return
    }
    if (style === saved) {
      onOpenChange(false)
      return
    }
    setSelected(style)
    mutation.mutate({ description_style: style })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <div className="flex flex-col gap-6">
          <SheetHeader>
            <SheetTitle>{t.descriptionStyleTitle}</SheetTitle>
            <SheetDescription>{t.descriptionStyleIntro}</SheetDescription>
          </SheetHeader>
          <fieldset
            disabled={mutation.isPending}
            aria-busy={mutation.isPending}
            className="flex min-w-0 flex-col gap-2"
          >
            <legend className="sr-only">{t.descriptionStyle}</legend>
            {descriptionStyles.map((style) => {
              const option = t.descriptionStyles[style]
              const checked = selected === style
              return (
                <label
                  key={style}
                  className="flex min-h-14 min-w-0 items-center gap-3 rounded-xl border border-border bg-card px-4 py-3 select-none has-checked:border-primary has-checked:bg-primary/8 has-checked:ring-2 has-checked:ring-primary has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-ring has-disabled:opacity-60"
                >
                  <input
                    type="radio"
                    name={name}
                    value={style}
                    checked={checked}
                    aria-labelledby={`${name}-${style}-label`}
                    aria-describedby={`${name}-${style}-example`}
                    // A click on the option already checked fires no change: it closes the sheet.
                    onClick={() => {
                      if (checked) {
                        choose(style)
                      }
                    }}
                    onChange={() => {
                      choose(style)
                    }}
                    className="sr-only"
                  />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span id={`${name}-${style}-label`} className="text-base">
                      {option.label}
                    </span>
                    <span
                      id={`${name}-${style}-example`}
                      className="text-sm [overflow-wrap:anywhere] text-muted-foreground"
                    >
                      {t.descriptionStyleExampleInput} <span aria-hidden="true">→</span>
                      <span className="sr-only">{t.descriptionStyleExampleBecomes}</span>{' '}
                      {option.example}
                    </span>
                  </span>
                  <CheckIcon
                    aria-hidden="true"
                    className={
                      checked ? 'size-5 shrink-0 text-primary' : 'invisible size-5 shrink-0'
                    }
                  />
                </label>
              )
            })}
          </fieldset>
          {mutation.isError && <FormAlert>{errorText(mutation.error)}</FormAlert>}
        </div>
      </SheetContent>
    </Sheet>
  )
}

interface SheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Lets a password manager tell which account the password belongs to. */
  email: string
}

function ChangePasswordSheet({
  open,
  onOpenChange,
  email,
  onChanged,
}: SheetProps & { onChanged: () => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <ChangePasswordForm email={email} onChanged={onChanged} />
      </SheetContent>
    </Sheet>
  )
}

function ChangePasswordForm({ email, onChanged }: { email: string; onChanged: () => void }) {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [currentError, setCurrentError] = useState<string | null>(null)
  const [nextError, setNextError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: changePassword,
    onSuccess: onChanged,
    onError: (err) => {
      // The new password is checked here first, so `invalid_input` is the current one
      // (ADR-0066: a wrong current password is 400, not 401).
      if (isApiError(err, 'invalid_input')) {
        setCurrentError(errorText(err))
      }
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mutation.isPending) {
      return
    }
    const cErr = current === '' ? common.enterPassword : null
    const nErr = passwordRuleError(next)
    setCurrentError(cErr)
    setNextError(nErr)
    if (cErr || nErr) {
      return
    }
    mutation.mutate({ current_password: current, new_password: next })
  }

  const formError =
    mutation.isError && !isApiError(mutation.error, 'invalid_input')
      ? errorText(mutation.error)
      : null

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.changePasswordTitle}</SheetTitle>
        <SheetDescription>{t.changePasswordIntro}</SheetDescription>
      </SheetHeader>
      <HiddenUsername email={email} />
      <TextField
        label={t.currentPassword}
        name="current-password"
        type="password"
        autoComplete="current-password"
        revealable
        value={current}
        error={currentError}
        onChange={(e) => {
          setCurrent(e.target.value)
        }}
      />
      <TextField
        label={t.newPassword}
        name="new-password"
        type="password"
        autoComplete="new-password"
        hint={common.passwordRule}
        revealable
        value={next}
        error={nextError}
        onChange={(e) => {
          setNext(e.target.value)
        }}
      />
      {formError && <FormAlert>{formError}</FormAlert>}
      <SheetFooter className="mt-0">
        <Button type="submit" size="lg" disabled={mutation.isPending} focusableWhenDisabled>
          {mutation.isPending ? t.savingPassword : t.savePassword}
        </Button>
      </SheetFooter>
    </form>
  )
}

function DeleteAccountSheet({
  open,
  onOpenChange,
  email,
  onDeleted,
}: SheetProps & { onDeleted: () => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <DeleteAccountForm email={email} onDeleted={onDeleted} />
      </SheetContent>
    </Sheet>
  )
}

function DeleteAccountForm({ email, onDeleted }: { email: string; onDeleted: () => void }) {
  const [password, setPassword] = useState('')
  const [fieldError, setFieldError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: deleteMe,
    onSuccess: onDeleted,
    onError: (err) => {
      if (isApiError(err, 'invalid_input')) {
        setFieldError(errorText(err))
      }
    },
  })

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (mutation.isPending) {
      return
    }
    const err = password === '' ? common.enterPassword : null
    setFieldError(err)
    if (err) {
      return
    }
    mutation.mutate({ password })
  }

  // `conflict` is the last operator (ADR-0068): the API's message says what to do.
  const formError =
    mutation.isError && !isApiError(mutation.error, 'invalid_input')
      ? errorText(mutation.error)
      : null

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <SheetHeader>
        <SheetTitle>{t.deleteTitle}</SheetTitle>
        <SheetDescription>{t.deleteWarning}</SheetDescription>
      </SheetHeader>
      <HiddenUsername email={email} />
      <TextField
        label={t.deletePassword}
        name="password"
        type="password"
        autoComplete="current-password"
        revealable
        value={password}
        error={fieldError}
        onChange={(e) => {
          setPassword(e.target.value)
        }}
      />
      {formError && <FormAlert>{formError}</FormAlert>}
      <SheetFooter className="mt-0">
        <SheetClose render={<Button variant="outline" size="lg" />}>{common.cancel}</SheetClose>
        <Button
          type="submit"
          variant="destructive"
          size="lg"
          disabled={mutation.isPending}
          focusableWhenDisabled
        >
          {mutation.isPending ? t.deleting : t.deleteSubmit}
        </Button>
      </SheetFooter>
    </form>
  )
}
