import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, json, mockApi, noContent } from '@/test/api'

import { login, logout, setPassword } from './auth'
import { ApiError, setTokenProvider } from './client'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok-old')
})

afterEach(() => {
  setTokenProvider(null)
})

const session = { token: 'tok-new', expires_at: '2026-11-02T03:00:00Z' }

describe('login', () => {
  it('posts the email and password without a bearer token and returns the session', async () => {
    api.on('POST /api/auth/login', () => json(200, session))

    await expect(login({ email: 'a@example.com', password: 'pw-123456789' })).resolves.toEqual(
      session,
    )

    const call = api.onlyCall('POST /api/auth/login')
    expect(call.body).toEqual({ email: 'a@example.com', password: 'pw-123456789' })
    expect(call.headers.has('Authorization')).toBe(false)
  })

  it.each([
    { name: 'no token', body: { expires_at: session.expires_at } },
    { name: 'an empty token', body: { token: '', expires_at: session.expires_at } },
    { name: 'a bad expiry', body: { token: 't', expires_at: 'soon' } },
    { name: 'not an object', body: ['t'] },
  ])('refuses a session with $name', async ({ body }) => {
    api.on('POST /api/auth/login', () => json(200, body))

    await expect(login({ email: 'a@example.com', password: 'x' })).rejects.toMatchObject({
      code: 'bad_response',
    })
  })

  it.each([
    { status: 401, code: 'unauthenticated', message: 'the email or password is incorrect' },
    {
      status: 429,
      code: 'rate_limited',
      message: 'too many failed attempts; try again in 15 minutes',
    },
  ])('passes $code through', async ({ status, code, message }) => {
    api.on('POST /api/auth/login', () => apiError(status, code, message))

    const err: unknown = await login({ email: 'a@example.com', password: 'x' }).catch(
      (e: unknown) => e,
    )

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status, code, message })
  })
})

describe('setPassword', () => {
  it('posts the link token and password without a bearer token', async () => {
    api.on('POST /api/auth/set-password', () => json(200, session))

    await expect(setPassword({ token: 'link-tok', password: 'long-enough-pw' })).resolves.toEqual(
      session,
    )

    const call = api.onlyCall('POST /api/auth/set-password')
    expect(call.body).toEqual({ token: 'link-tok', password: 'long-enough-pw' })
    expect(call.headers.has('Authorization')).toBe(false)
  })

  it('maps a bad link to invalid_input', async () => {
    api.on('POST /api/auth/set-password', () =>
      apiError(400, 'invalid_input', 'this link is invalid or has expired'),
    )

    await expect(setPassword({ token: 't', password: 'long-enough-pw' })).rejects.toMatchObject({
      code: 'invalid_input',
      message: 'this link is invalid or has expired',
    })
  })
})

describe('logout', () => {
  it('posts with the bearer token and no body', async () => {
    api.on('POST /api/auth/logout', () => noContent())

    await expect(logout()).resolves.toBeUndefined()

    const call = api.onlyCall('POST /api/auth/logout')
    expect(call.headers.get('Authorization')).toBe('Bearer tok-old')
    expect(call.body).toBeUndefined()
  })
})
