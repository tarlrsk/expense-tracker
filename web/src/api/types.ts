// Request and response shapes of the API, kept in step with docs/04-api.md and
// the Go handler types (ADR-0031).

/** GET /api/healthz (api/internal/handler/health). */
export interface HealthResponse {
  status: 'ok'
}

// Account (api/internal/handler/account; ADR-0066, ADR-0068). Timestamps are RFC 3339 text.

export const roles = ['user', 'operator'] as const
export type Role = (typeof roles)[number]

export const userStatuses = ['invited', 'active'] as const
export type UserStatus = (typeof userStatuses)[number]

/** Body of POST /api/auth/login and POST /api/auth/set-password. */
export interface Session {
  token: string
  expires_at: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface SetPasswordRequest {
  /** The token from the emailed link's fragment. */
  token: string
  password: string
}

/** Body of GET and PATCH /api/me. */
export interface Profile {
  id: string
  email: string
  /** Empty when the user has not set one. */
  display_name: string
  role: Role
  created_at: string
}

export interface UpdateMeRequest {
  /** 0 to 50 characters, trimmed by the API. */
  display_name: string
}

export interface DeleteMeRequest {
  password: string
}

export interface ChangePasswordRequest {
  current_password: string
  new_password: string
}

/** One account in the operator's list (GET /api/admin/users) and in an invite's answer. */
export interface UserItem {
  id: string
  email: string
  display_name: string
  role: Role
  status: UserStatus
  created_at: string
  /** null for a user who has never had a session. */
  last_active_at: string | null
}

export interface ListUsersResponse {
  users: UserItem[]
}

export interface InviteRequest {
  email: string
}

/** Body of POST /api/admin/invites (201), also when the email could not be sent. */
export interface InviteResponse {
  user: UserItem
  email_sent: boolean
}

/** Body of POST /api/admin/users/{id}/set-password-link. */
export interface EmailSentResponse {
  email_sent: boolean
}
