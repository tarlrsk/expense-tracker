/** A screen with nothing to show yet: what will be here, and why it is empty. */
export function EmptyState({ body, note }: { body: string; note: string }) {
  return (
    <div className="flex flex-col gap-2 border-y border-border py-8">
      <p className="text-base">{body}</p>
      <p className="text-base text-muted-foreground">{note}</p>
    </div>
  )
}
