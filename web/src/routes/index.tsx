import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { ApiError } from '@/api/client'
import { getHealth } from '@/api/health'
import { Button } from '@/components/ui/button'

export const Route = createFileRoute('/')({
  component: HomePage,
})

function HomePage() {
  const health = useQuery({
    queryKey: ['health'],
    queryFn: ({ signal }) => getHealth(signal),
    // The page has its own "check again" button.
    retry: false,
  })

  return (
    <div className="flex flex-col gap-6 py-6">
      <h1 className="text-2xl font-semibold">Expense Tracker</h1>

      <section aria-labelledby="server-status" className="flex flex-col gap-4">
        <h2 id="server-status" className="text-lg font-medium">
          Server status
        </h2>
        <p role="status" className="text-muted-foreground">
          {health.isPending && 'Checking the server…'}
          {health.isSuccess && 'The server is up.'}
          {health.isError && errorText(health.error)}
        </p>
        <Button
          size="lg"
          className="self-start"
          disabled={health.isFetching}
          onClick={() => void health.refetch()}
        >
          {health.isFetching ? 'Checking…' : 'Check again'}
        </Button>
      </section>
    </div>
  )
}

function errorText(error: Error): string {
  if (error instanceof ApiError && error.code === 'network') {
    return 'Could not reach the server.'
  }
  return 'The server is not responding correctly.'
}
