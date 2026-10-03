// Operator-only account management: /api/admin/* (docs/04-api.md, ADR-0068). These answers
// never carry other users' financial data.

import { apiRequest } from './client'
import {
  array,
  boolean,
  nonEmptyString,
  nullableTimestamp,
  object,
  oneOf,
  string,
  timestamp,
} from './shape'
import { roles, userStatuses } from './types'
import type {
  EmailSentResponse,
  InviteRequest,
  InviteResponse,
  ListUsersResponse,
  UserItem,
} from './types'

export function parseUserItem(value: unknown): UserItem {
  const o = object(value, 'user')
  return {
    id: nonEmptyString(o, 'id'),
    email: string(o, 'email'),
    display_name: string(o, 'display_name'),
    role: oneOf(o, 'role', roles),
    status: oneOf(o, 'status', userStatuses),
    created_at: timestamp(o, 'created_at'),
    last_active_at: nullableTimestamp(o, 'last_active_at'),
  }
}

export function parseListUsers(body: unknown): ListUsersResponse {
  const o = object(body, 'list')
  return { users: array(o.users, 'users').map(parseUserItem) }
}

export function parseInvite(body: unknown): InviteResponse {
  const o = object(body, 'invite')
  return { user: parseUserItem(o.user), email_sent: boolean(o, 'email_sent') }
}

export function parseEmailSent(body: unknown): EmailSentResponse {
  return { email_sent: boolean(object(body, 'link'), 'email_sent') }
}

/** GET /api/admin/users */
export function listUsers(signal?: AbortSignal): Promise<ListUsersResponse> {
  return apiRequest('/admin/users', { signal, parse: parseListUsers })
}

/** POST /api/admin/invites — 409 `conflict` when the email already has an account. */
export function inviteUser(req: InviteRequest): Promise<InviteResponse> {
  return apiRequest('/admin/invites', { method: 'POST', body: req, parse: parseInvite })
}

/** POST /api/admin/users/{id}/set-password-link — cancels the user's earlier links. */
export function sendSetPasswordLink(id: string): Promise<EmailSentResponse> {
  return apiRequest(`/admin/users/${encodeURIComponent(id)}/set-password-link`, {
    method: 'POST',
    parse: parseEmailSent,
  })
}

/**
 * DELETE /api/admin/users/{id} — removes the user and all their data. 409 `conflict` for the
 * operator's own id and for the last operator.
 */
export function removeUser(id: string): Promise<void> {
  return apiRequest<undefined>(`/admin/users/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
