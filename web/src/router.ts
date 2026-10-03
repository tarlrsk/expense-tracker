import { QueryClient } from '@tanstack/react-query'
import { createRouter } from '@tanstack/react-router'
import type { RouterHistory } from '@tanstack/react-router'

import { ApiError } from './api/client'
import { routeTree } from './routeTree.gen'
import { installSession } from './session/session'

/** Retries only failures that may pass on a second try; a 4xx answer will not change. */
export function createAppQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: (failureCount, err) =>
          failureCount < 2 &&
          err instanceof ApiError &&
          (err.code === 'network' || err.code === 'timeout' || err.code === 'internal'),
      },
    },
  })
}

/**
 * The app's router, with the session connected to the API client: a 401 on an authed call ends
 * the session and opens Login, which brings the person back here afterwards (ADR-0037).
 * `history` is for tests (a memory history); the browser's is used otherwise.
 */
export function createAppRouter(queryClient: QueryClient, history?: RouterHistory) {
  const router = createRouter({
    routeTree,
    context: { queryClient },
    history,
    scrollRestoration: true,
  })
  installSession(queryClient, () => {
    const { pathname, href } = router.state.location
    if (pathname === '/login') {
      return
    }
    void router.navigate({
      to: '/login',
      search: pathname === '/set-password' ? {} : { redirect: href },
      replace: true,
    })
  })
  return router
}

export type AppRouter = ReturnType<typeof createAppRouter>

declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter
  }
}
