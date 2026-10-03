// Public login endpoints and log out (docs/04-api.md, ADR-0066).

import { apiRequest } from './client'
import { nonEmptyString, object, timestamp } from './shape'
import type { LoginRequest, Session, SetPasswordRequest } from './types'

export function parseSession(body: unknown): Session {
  const o = object(body, 'session')
  return { token: nonEmptyString(o, 'token'), expires_at: timestamp(o, 'expires_at') }
}

/** POST /api/auth/login — public; a wrong email or password is 401 with one message. */
export function login(req: LoginRequest): Promise<Session> {
  return apiRequest('/auth/login', {
    method: 'POST',
    body: req,
    auth: false,
    parse: parseSession,
  })
}

/** POST /api/auth/set-password — public; answers with a new session (ADR-0066). */
export function setPassword(req: SetPasswordRequest): Promise<Session> {
  return apiRequest('/auth/set-password', {
    method: 'POST',
    body: req,
    auth: false,
    parse: parseSession,
  })
}

/** POST /api/auth/logout — ends the current session. */
export function logout(): Promise<void> {
  return apiRequest<undefined>('/auth/logout', { method: 'POST' })
}
