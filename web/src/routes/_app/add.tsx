import { createFileRoute } from '@tanstack/react-router'

import { EmptyState } from '@/components/empty-state'
import { PageTitle } from '@/components/layout'
import { placeholder } from '@/messages/placeholder'

// Built in PLAN-0002 T10; until then an honest empty state.
export const Route = createFileRoute('/_app/add')({
  component: AddPage,
})

function AddPage() {
  return (
    <>
      <PageTitle>{placeholder.addTitle}</PageTitle>
      <EmptyState body={placeholder.addBody} note={placeholder.addNotYet} />
    </>
  )
}
