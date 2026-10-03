import { createFileRoute } from '@tanstack/react-router'

import { EmptyState } from '@/components/empty-state'
import { PageTitle } from '@/components/layout'
import { placeholder } from '@/messages/placeholder'

// Built in PLAN-0002 T10; until then an honest empty state, no sample data.
export const Route = createFileRoute('/_app/history')({
  component: HistoryPage,
})

function HistoryPage() {
  return (
    <>
      <PageTitle>{placeholder.historyTitle}</PageTitle>
      <EmptyState body={placeholder.historyBody} note={placeholder.historyNotYet} />
    </>
  )
}
