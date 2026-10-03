import { Link } from '@tanstack/react-router'
import { cn } from 'cn'
import { PlusIcon, ReceiptTextIcon, SettingsIcon } from 'lucide-react'

import { nav } from '@/messages/nav'

// The app's three places (ADR-0072): History, Add (the larger middle button), Settings. On a
// phone the bar sits at the bottom, within thumb reach and clear of the home indicator; from
// `md` up it is at the top of the same centred column. Shown only when logged in (_app route).

const placeClass =
  'flex h-full min-h-11 flex-col items-center justify-center gap-1 rounded-xl text-sm text-muted-foreground select-none data-[status=active]:font-semibold data-[status=active]:text-primary md:flex-row md:gap-2 md:px-3 md:text-base'

export function AppBar() {
  return (
    <nav
      aria-label={nav.label}
      className="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background pr-[env(safe-area-inset-right)] pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)] md:sticky md:top-0 md:bottom-auto md:border-t-0 md:border-b md:pt-[env(safe-area-inset-top)]"
    >
      <ul className="mx-auto grid h-16 w-full max-w-xl grid-cols-3 items-center gap-2 px-2">
        <li className="h-full py-1">
          <Link to="/history" className={placeClass}>
            <ReceiptTextIcon aria-hidden="true" className="size-6 md:size-5" />
            {nav.history}
          </Link>
        </li>
        <li className="flex justify-center">
          <Link
            to="/add"
            className={cn(
              'flex h-12 items-center gap-2 rounded-full bg-primary pr-5 pl-4 text-base font-semibold text-primary-foreground shadow-[0_2px_8px_rgb(31_122_84/0.35)] select-none active:translate-y-px data-[status=active]:ring-3 data-[status=active]:ring-primary/30',
            )}
          >
            <PlusIcon aria-hidden="true" className="size-6" strokeWidth={2.5} />
            {nav.add}
          </Link>
        </li>
        <li className="h-full py-1">
          <Link to="/settings" className={placeClass}>
            <SettingsIcon aria-hidden="true" className="size-6 md:size-5" />
            {nav.settings}
          </Link>
        </li>
      </ul>
    </nav>
  )
}
