import { cn } from 'cn'
import { useId } from 'react'

export interface Choice<T extends string> {
  value: T
  label: string
}

/**
 * A switch between a few choices, such as Expense / Income or Week / Month: native radio
 * buttons (arrow keys move between them, a screen reader names the group) drawn as one
 * segmented control. Each segment is 44px high.
 */
export function ChoiceGroup<T extends string>({
  legend,
  choices,
  value,
  onChange,
  showLegend = false,
  className,
}: {
  /** Read by screen readers; shown only with `showLegend` (the choices often speak for themselves). */
  legend: string
  showLegend?: boolean
  choices: readonly Choice<T>[]
  value: T
  onChange: (value: T) => void
  className?: string
}) {
  const name = useId()
  return (
    <fieldset className={cn('min-w-0', className)}>
      <legend className={showLegend ? 'mb-2 text-base leading-snug font-semibold' : 'sr-only'}>
        {legend}
      </legend>
      <div className="grid auto-cols-fr grid-flow-col gap-1 rounded-xl bg-muted p-1">
        {choices.map((choice) => (
          <label
            key={choice.value}
            className="flex h-11 items-center justify-center rounded-lg px-2 text-base text-muted-foreground select-none has-checked:bg-card has-checked:font-semibold has-checked:text-foreground has-checked:shadow-[0_1px_3px_rgb(18_38_31/0.12)] has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-ring"
          >
            <input
              type="radio"
              name={name}
              value={choice.value}
              checked={value === choice.value}
              onChange={() => {
                onChange(choice.value)
              }}
              className="sr-only"
            />
            {choice.label}
          </label>
        ))}
      </div>
    </fieldset>
  )
}
