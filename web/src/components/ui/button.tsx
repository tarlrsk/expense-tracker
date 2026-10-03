import { Button as ButtonPrimitive } from '@base-ui/react/button'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from 'cn'

// Adapted to the design plan (ADR-0072): 16px semibold labels, every size at least 44px high,
// no hover-only meaning, no text selection, and a pressed state for touch.
const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center gap-2 rounded-xl border border-transparent bg-clip-padding text-base font-semibold whitespace-nowrap transition-[background-color,color,transform] duration-100 outline-none select-none focus-visible:ring-3 focus-visible:ring-ring/40 active:translate-y-px disabled:pointer-events-none disabled:opacity-60 aria-disabled:pointer-events-none aria-disabled:opacity-60 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-5",
  {
    variants: {
      variant: {
        default:
          'bg-primary text-primary-foreground hover:bg-[color-mix(in_oklch,var(--primary),black_15%)] active:bg-[color-mix(in_oklch,var(--primary),black_15%)]',
        outline: 'border-input bg-card text-foreground hover:bg-muted active:bg-muted',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-border active:bg-border',
        ghost: 'text-foreground hover:bg-muted active:bg-muted',
        // Red is only for deleting (ADR-0072).
        destructive:
          'bg-destructive text-destructive-foreground hover:bg-[color-mix(in_oklch,var(--destructive),black_15%)] focus-visible:ring-destructive/40 active:bg-[color-mix(in_oklch,var(--destructive),black_15%)]',
        'destructive-ghost':
          'text-destructive hover:bg-destructive/8 focus-visible:ring-destructive/40 active:bg-destructive/8',
      },
      size: {
        default: 'h-11 px-4',
        lg: 'h-12 px-5',
        sm: 'h-11 px-3',
        icon: 'size-11',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

function Button({
  className,
  variant = 'default',
  size = 'default',
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
