// Typed fetch wrapper for the Go API (ADR-0031). Every call goes to /api on the
// same origin, sends no cookies, and adds the bearer token only when one exists.

/** Error codes sent by the API (ADR-0046, docs/04-api.md "Error codes"). */
export const serverErrorCodes = [
  'invalid_input',
  'unauthenticated',
  'forbidden',
  'not_found',
  'conflict',
  'rate_limited',
  'timeout',
  'internal',
] as const

export type ServerErrorCode = (typeof serverErrorCodes)[number]

/**
 * Every code an ApiError can carry: the API's codes plus two client-only ones.
 * - `network`: the request never got a response (offline, server down).
 * - `bad_response`: the response was not the shape the client expects.
 */
export type ApiErrorCode = ServerErrorCode | 'network' | 'bad_response'

export class ApiError extends Error {
  override readonly name = 'ApiError'
  /** HTTP status; 0 when there was no response. */
  readonly status: number
  readonly code: ApiErrorCode

  constructor(status: number, code: ApiErrorCode, message: string, options?: ErrorOptions) {
    super(message, options)
    this.status = status
    this.code = code
  }
}

export type TokenProvider = () => string | null

let tokenProvider: TokenProvider = () => null

/** Sets where the client reads the session token from; null removes it. */
export function setTokenProvider(provider: TokenProvider | null): void {
  tokenProvider = provider ?? (() => null)
}

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export interface RequestOptions {
  method?: HttpMethod
  /** Sent as JSON. */
  body?: unknown
  signal?: AbortSignal
}

const basePath = '/api'

/**
 * Calls `/api{path}` and returns the parsed JSON body, or undefined for 204.
 * Throws ApiError for any failure except an aborted request, whose AbortError
 * is passed through unchanged.
 */
export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, signal } = options

  const headers = new Headers({ Accept: 'application/json' })
  const token = tokenProvider()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const init: RequestInit = { method, headers, credentials: 'omit', signal }
  if (body !== undefined) {
    headers.set('Content-Type', 'application/json')
    init.body = JSON.stringify(body)
  }

  let response: Response
  try {
    response = await fetch(basePath + path, init)
  } catch (err) {
    if (isAbortError(err)) {
      throw err
    }
    throw new ApiError(0, 'network', 'Could not reach the server.', { cause: err })
  }

  if (!response.ok) {
    throw await errorFromResponse(response)
  }
  if (response.status === 204) {
    return undefined as T
  }

  try {
    return (await response.json()) as T
  } catch (err) {
    if (isAbortError(err)) {
      throw err
    }
    throw badResponse(response.status, err)
  }
}

async function errorFromResponse(response: Response): Promise<ApiError> {
  let parsed: unknown
  try {
    parsed = await response.json()
  } catch (err) {
    return badResponse(response.status, err)
  }
  if (!isErrorBody(parsed)) {
    return badResponse(response.status)
  }
  return new ApiError(response.status, parsed.error.code, parsed.error.message)
}

function badResponse(status: number, cause?: unknown): ApiError {
  return new ApiError(
    status,
    'bad_response',
    `Unexpected response from the server (status ${String(status)}).`,
    cause === undefined ? undefined : { cause },
  )
}

interface ErrorBody {
  error: { code: ServerErrorCode; message: string }
}

function isErrorBody(value: unknown): value is ErrorBody {
  if (typeof value !== 'object' || value === null || !('error' in value)) {
    return false
  }
  const { error } = value
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    'message' in error &&
    isServerErrorCode(error.code) &&
    typeof error.message === 'string'
  )
}

function isServerErrorCode(code: unknown): code is ServerErrorCode {
  return typeof code === 'string' && (serverErrorCodes as readonly string[]).includes(code)
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}
