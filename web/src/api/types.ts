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

// Categories (api/internal/handler/categories; ADR-0039, ADR-0061, ADR-0069).

export const categoryKinds = ['expense', 'income'] as const
export type CategoryKind = (typeof categoryKinds)[number]

/** One category; GET /api/categories lists archived ones too, in the user's order. */
export interface Category {
  id: string
  name: string
  /** An emoji, or empty when the category has none. */
  icon: string
  kind: CategoryKind
  archived: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

/** Body of GET /api/categories and PUT /api/categories/order. */
export interface ListCategoriesResponse {
  categories: Category[]
}

/** Body of POST /api/categories: added at the end. */
export interface CreateCategoryRequest {
  /** 1 to 50 characters; the API trims it and collapses inner whitespace. */
  name: string
  kind: CategoryKind
  /** At most 32 characters; trimmed by the API. */
  icon?: string
}

/** Body of PATCH /api/categories/{id}: only the fields sent change; `kind` never does. */
export interface UpdateCategoryRequest {
  name?: string
  icon?: string
  archived?: boolean
}

/** Body of PUT /api/categories/order: every non-archived id exactly once. */
export interface ReorderCategoriesRequest {
  ids: string[]
}

// Transactions (api/internal/handler/transactions; ADR-0040, ADR-0071). Amounts are text such
// as "145.00", never numbers; dates are YYYY-MM-DD.

export const transactionSources = ['manual', 'text', 'scan', 'csv'] as const
export type TransactionSource = (typeof transactionSources)[number]

export interface Transaction {
  id: string
  owner_id: string
  amount: string
  currency: string
  occurred_on: string
  /** Empty when there is none. */
  merchant: string
  category_id: string
  /** Empty when there is none. */
  note: string
  source: TransactionSource
  created_at: string
  updated_at: string
}

/** Body of GET /api/transactions: newest first; `next_cursor` is null on the last page. */
export interface ListTransactionsResponse {
  transactions: Transaction[]
  next_cursor: string | null
}

/** The period of a list: a month (`YYYY-MM`), or a date range with both ends included. */
export type TransactionPeriod = { month: string } | { from?: string; to?: string }

export type ListTransactionsParams = TransactionPeriod & {
  /** 1 to 200; the API's default is 50. */
  limit?: number
  cursor?: string
}

/** Body of POST /api/transactions; `id` is a UUID v7 made by the app (ADR-0040). */
export interface CreateTransactionRequest {
  id: string
  amount: string
  occurred_on: string
  category_id: string
  merchant?: string
  note?: string
}

/** Body of PATCH /api/transactions/{id}: only the fields sent change. */
export interface UpdateTransactionRequest {
  amount?: string
  occurred_on?: string
  category_id?: string
  merchant?: string
  note?: string
}
