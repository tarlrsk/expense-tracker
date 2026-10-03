import { Outlet, createFileRoute, redirect } from '@tanstack/react-router'

import { AppBar } from '@/components/app-bar'
import { isLoggedIn } from '@/session/session'

// Every app screen is under this layout: a logged-out visitor lands on Login and comes back to
// where they were going (ADR-0037).
export const Route = createFileRoute('/_app')({
  beforeLoad: ({ location }) => {
    if (!isLoggedIn()) {
      throw redirect({ to: '/login', search: { redirect: location.href } })
    }
  },
  component: AppLayout,
})

function AppLayout() {
  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar />
      {/* On a phone the bar is fixed at the bottom, so the content stops above it. */}
      <main className="mx-auto flex w-full max-w-xl flex-1 flex-col gap-6 pt-[max(1rem,env(safe-area-inset-top))] pr-[max(1rem,env(safe-area-inset-right))] pb-[calc(5rem+env(safe-area-inset-bottom))] pl-[max(1rem,env(safe-area-inset-left))] md:pt-8 md:pb-12">
        <Outlet />
      </main>
    </div>
  )
}
