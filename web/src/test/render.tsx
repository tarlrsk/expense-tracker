import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router'
import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { createAppQueryClient, createAppRouter } from '@/router'
import { setToken } from '@/session/token'

/** Renders the whole app at `path`, with the real routes and session, on a memory history. */
export function renderApp(path: string, { token }: { token?: string } = {}) {
  if (token) {
    setToken(token)
  }
  const queryClient = createAppQueryClient()
  // Tests see the first failure at once.
  queryClient.setDefaultOptions({ queries: { retry: false } })
  const history = createMemoryHistory({ initialEntries: [path] })
  const router = createAppRouter(queryClient, history)
  const user = userEvent.setup()
  const view = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { ...view, user, router, queryClient, history }
}
