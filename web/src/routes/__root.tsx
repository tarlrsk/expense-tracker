import type { QueryClient } from '@tanstack/react-query'
import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
})

// Phone-first shell (ADR-0050): one column that fills a phone, centred with a
// max width on wider screens, clear of notches and home indicators.
function RootLayout() {
  return (
    <div className="mx-auto flex min-h-dvh w-full flex-col pad-safe sm:max-w-xl">
      <main className="flex flex-1 flex-col">
        <Outlet />
      </main>
    </div>
  )
}
