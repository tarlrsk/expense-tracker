import type { QueryClient } from '@tanstack/react-query'
import { Link, Outlet, createRootRouteWithContext, useRouter } from '@tanstack/react-router'
import type { ErrorComponentProps } from '@tanstack/react-router'

import { FormAlert } from '@/components/messages'
import { Button } from '@/components/ui/button'
import { errorText } from '@/lib/errors'
import { common } from '@/messages/common'

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: Outlet,
  errorComponent: RootError,
  notFoundComponent: NotFound,
})

// Phone-first (ADR-0050): every screen fills a phone and is centred with a max width on wider
// screens; the layouts keep content clear of notches and home indicators.

function RootError({ error }: ErrorComponentProps) {
  const router = useRouter()
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center gap-6 pad-safe">
      <h1 className="text-xl font-semibold">{common.errorTitle}</h1>
      <FormAlert>{errorText(error)}</FormAlert>
      <Button
        size="lg"
        onClick={() => {
          void router.invalidate()
        }}
      >
        {common.tryAgain}
      </Button>
    </main>
  )
}

function NotFound() {
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center gap-6 pad-safe">
      <h1 className="text-xl font-semibold">{common.pageNotFound}</h1>
      <Link
        to="/history"
        className="flex h-12 items-center justify-center rounded-xl bg-primary px-5 text-base font-semibold text-primary-foreground select-none"
      >
        {common.goHome}
      </Link>
    </main>
  )
}
