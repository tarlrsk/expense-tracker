// Starting and ending a session in the browser (ADR-0037, ADR-0066). The profile from
// GET /api/me is the source of "who am I" and of the role; it lives in the query cache.

import { queryOptions } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'

import { logout } from '@/api/auth'
import { setTokenProvider, setUnauthenticatedHandler } from '@/api/client'
import { getMe } from '@/api/me'

import { clearToken, getToken, setToken } from './token'

export const meQuery = queryOptions({
  queryKey: ['me'],
  queryFn: ({ signal }) => getMe(signal),
})

export function isLoggedIn(): boolean {
  return getToken() !== null
}

/** Stores a new session's token; anything cached for an earlier session is dropped. */
export function startSession(queryClient: QueryClient, token: string): void {
  queryClient.clear()
  setToken(token)
}

/** Forgets the session in this browser: the token and every cached answer. */
export function endSession(queryClient: QueryClient): void {
  clearToken()
  queryClient.clear()
}

/** Logs out: tells the API, then ends the session here even if the call failed. */
export async function logOut(queryClient: QueryClient): Promise<void> {
  try {
    await logout()
  } catch {
    // The token is forgotten below either way; an unreachable server must not keep the
    // person logged in on this device.
  } finally {
    endSession(queryClient)
  }
}

/**
 * Connects the API client to the session: it reads the stored token, and a 401 on an authed
 * call ends the session and calls `toLogin` (which sends the person to Login).
 */
export function installSession(queryClient: QueryClient, toLogin: () => void): void {
  setTokenProvider(getToken)
  setUnauthenticatedHandler(() => {
    endSession(queryClient)
    toLogin()
  })
}
