import { createFileRoute, redirect } from '@tanstack/react-router'

// The app opens on History.
export const Route = createFileRoute('/_app/')({
  beforeLoad: () => {
    throw redirect({ to: '/history', replace: true })
  },
})
