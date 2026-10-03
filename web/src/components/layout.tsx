import { Link } from '@tanstack/react-router'
import { cn } from 'cn'
import { ChevronLeftIcon } from 'lucide-react'
import type * as React from 'react'

import { common } from '@/messages/common'

/** The name of the app; the small brass coin is the only ornament. */
export function Wordmark() {
  return (
    <p className="flex items-center gap-2 text-xl font-semibold text-foreground">
      <span aria-hidden="true" className="size-3 rounded-full bg-brass" />
      {common.appName}
    </p>
  )
}

/**
 * Login and Set password: one column with the form low on a phone screen, within thumb reach,
 * and centred from `md` up. No app bar: the person is not logged in.
 */
export function PublicLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-md flex-col pad-safe">
      <header className="py-2">
        <Wordmark />
      </header>
      <main className="flex flex-1 flex-col justify-end pt-8 md:justify-center">{children}</main>
    </div>
  )
}

/** The rounded surface a form sits in (ADR-0072). */
export function FormSheet({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      className={cn(
        'flex flex-col gap-6 rounded-2xl border border-border bg-card p-4 md:p-6',
        className,
      )}
      {...props}
    />
  )
}

export function PageTitle({ children }: { children: React.ReactNode }) {
  return <h1 className="text-display font-semibold">{children}</h1>
}

/** A 44px link back to the place a screen was opened from. */
export function BackLink({ to, label }: { to: '/settings'; label: string }) {
  return (
    <Link
      to={to}
      className="-ml-2 flex h-11 items-center gap-1 self-start rounded-xl pr-3 pl-1 text-base text-primary select-none"
    >
      <ChevronLeftIcon aria-hidden="true" className="size-5" />
      <span>
        <span className="sr-only">{common.back}: </span>
        {label}
      </span>
    </Link>
  )
}
