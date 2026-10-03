import { cn } from 'cn'
import { EyeIcon, EyeOffIcon } from 'lucide-react'
import { useId, useState } from 'react'
import type * as React from 'react'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { common } from '@/messages/common'

type InputProps = Omit<React.ComponentProps<'input'>, 'id' | 'aria-invalid' | 'aria-describedby'>

export interface TextFieldProps extends InputProps {
  label: string
  /** Shown under the label before anything is typed, e.g. a rule such as "At least 10 characters." */
  hint?: string
  /** Shown next to the field and announced; marks the input invalid. */
  error?: string | null
  /** For a password: adds a show / hide control. */
  revealable?: boolean
}

/**
 * A labelled input with its hint and error tied to it (`aria-describedby`), so a screen reader
 * reads them with the field. The error is in a live region, so it is announced when it appears.
 */
export function TextField({
  label,
  hint,
  error,
  revealable = false,
  className,
  type,
  ...props
}: TextFieldProps) {
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const [revealed, setRevealed] = useState(false)
  const describedBy = [hint ? hintId : null, error ? errorId : null].filter(Boolean).join(' ')

  return (
    <div className={cn('flex flex-col gap-2', className)}>
      <Label htmlFor={id}>{label}</Label>
      {hint && (
        <p id={hintId} className="-mt-1 text-sm text-muted-foreground">
          {hint}
        </p>
      )}
      <div className="relative">
        <Input
          id={id}
          type={revealable && revealed ? 'text' : type}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy || undefined}
          className={revealable ? 'pr-12' : undefined}
          {...props}
        />
        {revealable && (
          <button
            type="button"
            className="absolute inset-y-0 right-0 flex w-12 items-center justify-center rounded-r-xl text-muted-foreground select-none hover:text-foreground focus-visible:outline-offset-[-4px]"
            aria-pressed={revealed}
            aria-label={revealed ? common.hidePassword : common.showPassword}
            aria-controls={id}
            onClick={() => {
              setRevealed((r) => !r)
            }}
          >
            {revealed ? <EyeOffIcon className="size-5" /> : <EyeIcon className="size-5" />}
          </button>
        )}
      </div>
      <p id={errorId} aria-live="polite" className="text-sm text-destructive empty:hidden">
        {error ?? ''}
      </p>
    </div>
  )
}
