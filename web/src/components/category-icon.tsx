import { cn } from 'cn'
import { TagIcon } from 'lucide-react'

/**
 * A category's emoji (ADR-0072) in a round muted tile; a plain tag when it has none. Decorative:
 * the category's name is always written next to it.
 */
export function CategoryIcon({ icon, className }: { icon: string; className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-xl leading-none',
        className,
      )}
    >
      {icon === '' ? <TagIcon className="size-5 text-muted-foreground" /> : icon}
    </span>
  )
}
