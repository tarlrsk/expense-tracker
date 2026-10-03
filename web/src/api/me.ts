// The caller's own account: /api/me (docs/04-api.md, ADR-0066, ADR-0068).

import { apiRequest } from './client'
import { nonEmptyString, object, oneOf, string, timestamp } from './shape'
import { roles } from './types'
import type { ChangePasswordRequest, DeleteMeRequest, Profile, UpdateMeRequest } from './types'

export function parseProfile(body: unknown): Profile {
  const o = object(body, 'profile')
  return {
    id: nonEmptyString(o, 'id'),
    email: string(o, 'email'),
    display_name: string(o, 'display_name'),
    role: oneOf(o, 'role', roles),
    created_at: timestamp(o, 'created_at'),
  }
}

/** GET /api/me — who is logged in, and their role. */
export function getMe(signal?: AbortSignal): Promise<Profile> {
  return apiRequest('/me', { signal, parse: parseProfile })
}

/** PATCH /api/me — returns the profile after the change. */
export function updateMe(req: UpdateMeRequest): Promise<Profile> {
  return apiRequest('/me', { method: 'PATCH', body: req, parse: parseProfile })
}

/**
 * DELETE /api/me — deletes the account and all its data (ADR-0038). A wrong password is 400
 * `invalid_input`; the last operator is 409 `conflict` (ADR-0068).
 */
export function deleteMe(req: DeleteMeRequest): Promise<void> {
  return apiRequest<undefined>('/me', { method: 'DELETE', body: req })
}

/**
 * POST /api/me/password — ends the user's other sessions. A wrong current password is 400
 * `invalid_input`, not 401 (ADR-0066).
 */
export function changePassword(req: ChangePasswordRequest): Promise<void> {
  return apiRequest<undefined>('/me/password', { method: 'POST', body: req })
}
