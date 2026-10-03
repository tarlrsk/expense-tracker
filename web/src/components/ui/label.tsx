import { cn } from 'cn'
import type * as React from 'react'

// Adapted to the design plan (ADR-0072): 16px semibold, sentence case.
function Label({ className, ...props }: React.ComponentProps<'label'>) {
  return (
    <label
      data-slot="label"
      className={cn(
        'flex items-center gap-2 text-base leading-snug font-semibold select-none peer-disabled:cursor-not-allowed peer-disabled:opacity-60',
        className,
      )}
      {...props}
    />
  )
}

export { Label }
