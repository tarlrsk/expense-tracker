import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, apiRequest, serverErrorCodes, setTokenProvider } from './client'
import type { ApiErrorCode } from './client'
import { getHealth } from './health'

const fetchMock = vi.fn<typeof fetch>()

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function lastCall(): { url: string; init: RequestInit; headers: Headers } {
  const call = fetchMock.mock.lastCall
  if (!call) {
    throw new Error('fetch was not called')
  }
  const [input, init = {}] = call
  const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
  return { url, init, headers: new Headers(init.headers) }
}

async function catchApiError(promise: Promise<unknown>): Promise<ApiError> {
  try {
    await promise
  } catch (err) {
    expect(err).toBeInstanceOf(ApiError)
    return err as ApiError
  }
  throw new Error('expected the request to fail')
}

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
  setTokenProvider(null)
})

afterEach(() => {
  vi.unstubAllGlobals()
  setTokenProvider(null)
})

describe('apiRequest', () => {
  it('calls /api with JSON accept, no cookies, and returns the body', async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))

    await expect(getHealth()).resolves.toEqual({ status: 'ok' })

    const { url, init, headers } = lastCall()
    expect(url).toBe('/api/healthz')
    expect(init.method).toBe('GET')
    expect(init.credentials).toBe('omit')
    expect(headers.get('Accept')).toBe('application/json')
    expect(init.body).toBeUndefined()
    expect(headers.has('Content-Type')).toBe(false)
  })

  it.each([
    { name: 'a token', token: 'tok-123', want: 'Bearer tok-123' },
    { name: 'no token', token: null, want: null },
    { name: 'an empty token', token: '', want: null },
  ])('sends the bearer header only with a token: $name', async ({ token, want }) => {
    setTokenProvider(() => token)
    fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))

    await apiRequest('/healthz')

    expect(lastCall().headers.get('Authorization')).toBe(want)
  })

  it('sends no bearer header when no provider is set', async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))

    await apiRequest('/healthz')

    expect(lastCall().headers.has('Authorization')).toBe(false)
  })

  it('sends a JSON body and Content-Type on POST', async () => {
    fetchMock.mockResolvedValue(jsonResponse(201, { id: 'a' }))
    const body = { name: 'Food', amount: 120 }

    await expect(apiRequest('/categories', { method: 'POST', body })).resolves.toEqual({ id: 'a' })

    const { url, init, headers } = lastCall()
    expect(url).toBe('/api/categories')
    expect(init.method).toBe('POST')
    expect(headers.get('Content-Type')).toBe('application/json')
    expect(init.body).toBe(JSON.stringify(body))
  })

  it('returns undefined for 204 without reading a body', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }))

    await expect(
      apiRequest<undefined>('/transactions/x', { method: 'DELETE' }),
    ).resolves.toBeUndefined()
  })

  it('passes the abort signal to fetch', async () => {
    fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))
    const controller = new AbortController()

    await getHealth(controller.signal)

    expect(lastCall().init.signal).toBe(controller.signal)
  })

  it('passes an AbortError through unchanged', async () => {
    const abort = new DOMException('The operation was aborted.', 'AbortError')
    fetchMock.mockRejectedValue(abort)

    await expect(apiRequest('/healthz')).rejects.toBe(abort)
  })
})

describe('apiRequest errors', () => {
  const statusByCode: Record<(typeof serverErrorCodes)[number], number> = {
    invalid_input: 400,
    unauthenticated: 401,
    forbidden: 403,
    not_found: 404,
    conflict: 409,
    rate_limited: 429,
    timeout: 504,
    internal: 500,
  }

  it.each(serverErrorCodes.map((code) => ({ code, status: statusByCode[code] })))(
    'maps $status $code to ApiError',
    async ({ code, status }) => {
      fetchMock.mockResolvedValue(
        jsonResponse(status, { error: { code, message: 'safe message' } }),
      )

      const err = await catchApiError(apiRequest('/healthz'))

      expect(err.status).toBe(status)
      expect(err.code).toBe(code)
      expect(err.message).toBe('safe message')
    },
  )

  it.each<{ name: string; response: () => Response; status: number }>([
    {
      name: 'non-JSON error body',
      response: () => new Response('<html>Bad gateway</html>', { status: 502 }),
      status: 502,
    },
    { name: 'empty error body', response: () => new Response(null, { status: 500 }), status: 500 },
    {
      name: 'JSON without the error object',
      response: () => jsonResponse(400, { message: 'nope' }),
      status: 400,
    },
    {
      name: 'unknown error code',
      response: () => jsonResponse(418, { error: { code: 'teapot', message: 'short and stout' } }),
      status: 418,
    },
    {
      name: 'non-string message',
      response: () => jsonResponse(404, { error: { code: 'not_found', message: 42 } }),
      status: 404,
    },
    {
      name: 'non-JSON success body',
      response: () => new Response('ok', { status: 200 }),
      status: 200,
    },
  ])('maps a $name to bad_response', async ({ response, status }) => {
    fetchMock.mockResolvedValue(response())

    const err = await catchApiError(apiRequest('/healthz'))

    expect(err.code).toBe<ApiErrorCode>('bad_response')
    expect(err.status).toBe(status)
  })

  it('maps a rejected fetch to network with status 0', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'))

    const err = await catchApiError(apiRequest('/healthz'))

    expect(err.code).toBe<ApiErrorCode>('network')
    expect(err.status).toBe(0)
  })

  it('never puts the token or the request body in the error message', async () => {
    const token = 'secret-token-value'
    const body = { password: 'secret-password-value' }
    setTokenProvider(() => token)

    const failures: (() => void)[] = [
      () => fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch')),
      () => fetchMock.mockResolvedValueOnce(new Response('not json', { status: 500 })),
      () =>
        fetchMock.mockResolvedValueOnce(
          jsonResponse(401, { error: { code: 'unauthenticated', message: 'log in again' } }),
        ),
    ]
    for (const fail of failures) {
      fail()
      const err = await catchApiError(apiRequest('/auth/login', { method: 'POST', body }))
      expect(err.message).not.toContain(token)
      expect(err.message).not.toContain(body.password)
    }
  })
})
