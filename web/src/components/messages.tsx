import { cn } from 'cn'
import { CircleAlertIcon, CircleCheckIcon } from 'lucide-react'
import type * as React from 'react'

/** An error about a whole form or screen; announced at once (`role="alert"`). */
export function FormAlert({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div
      role="alert"
      className={cn(
        'flex gap-3 rounded-xl bg-destructive/8 px-4 py-3 text-base text-destructive',
        className,
      )}
    >
      <CircleAlertIcon aria-hidden="true" className="mt-0.5 size-5 shrink-0" />
      <div className="flex flex-col gap-1">{children}</div>
    </div>
  )
}

/**
 * The result of an action, such as "Name saved."; announced politely. The region is always in
 * the page so screen readers notice when text appears in it.
 */
export function Notice({
  children,
  className,
}: {
  children?: React.ReactNode
  className?: string
}) {
  return (
    <div role="status" className={cn(children ? undefined : 'sr-only', className)}>
      {children && (
        <div className="flex gap-3 rounded-xl bg-primary/8 px-4 py-3 text-base text-foreground">
          <CircleCheckIcon aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-primary" />
          <div className="flex flex-col gap-1">{children}</div>
        </div>
      )}
    </div>
  )
}
