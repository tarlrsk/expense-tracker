import { QueryClient } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { getMe } from '@/api/me'
import { setTokenProvider, setUnauthenticatedHandler } from '@/api/client'
import { apiError, json, mockApi, noContent, profile } from '@/test/api'

import { endSession, installSession, isLoggedIn, logOut, startSession } from './session'
import { clearToken, getToken, setToken, tokenStorageKey } from './token'

let api: ReturnType<typeof mockApi>
let queryClient: QueryClient

beforeEach(() => {
  api = mockApi()
  queryClient = new QueryClient()
})

afterEach(() => {
  setTokenProvider(null)
  setUnauthenticatedHandler(null)
})

describe('token store', () => {
  it('keeps the token under one localStorage key', () => {
    expect(getToken()).toBeNull()
    setToken('tok-1')
    expect(localStorage.getItem(tokenStorageKey)).toBe('tok-1')
    expect(localStorage.length).toBe(1)
    expect(getToken()).toBe('tok-1')
    expect(isLoggedIn()).toBe(true)
    clearToken()
    expect(getToken()).toBeNull()
    expect(isLoggedIn()).toBe(false)
  })

  it('reads an empty value as logged out', () => {
    localStorage.setItem(tokenStorageKey, '')
    expect(getToken()).toBeNull()
  })
})

describe('starting and ending a session', () => {
  it('startSession stores the token and drops the previous cache', () => {
    queryClient.setQueryData(['me'], { id: 'someone-else' })

    startSession(queryClient, 'tok-new')

    expect(getToken()).toBe('tok-new')
    expect(queryClient.getQueryData(['me'])).toBeUndefined()
  })

  it('endSession clears the token and the cache', () => {
    setToken('tok')
    queryClient.setQueryData(['me'], profile)

    endSession(queryClient)

    expect(getToken()).toBeNull()
    expect(queryClient.getQueryData(['me'])).toBeUndefined()
  })
})

describe('logOut', () => {
  it('tells the API, then clears the token and the cache', async () => {
    installSession(queryClient, vi.fn())
    setToken('tok')
    queryClient.setQueryData(['me'], profile)
    api.on('POST /api/auth/logout', () => noContent())

    await logOut(queryClient)

    const call = api.onlyCall('POST /api/auth/logout')
    expect(call.headers.get('Authorization')).toBe('Bearer tok')
    expect(getToken()).toBeNull()
    expect(queryClient.getQueryData(['me'])).toBeUndefined()
  })

  it.each([
    { name: 'the server cannot be reached', fail: () => Promise.reject(new TypeError('offline')) },
    { name: 'the server fails', fail: () => apiError(500, 'internal', 'internal error') },
  ])('still clears everything when $name', async ({ fail }) => {
    installSession(queryClient, vi.fn())
    setToken('tok')
    queryClient.setQueryData(['me'], profile)
    api.on('POST /api/auth/logout', fail)

    await logOut(queryClient)

    expect(getToken()).toBeNull()
    expect(queryClient.getQueryData(['me'])).toBeUndefined()
  })
})

describe('installSession', () => {
  it('gives the client the stored token', async () => {
    installSession(queryClient, vi.fn())
    setToken('tok-a')
    api.on('GET /api/me', () => json(200, profile))

    await getMe()

    expect(api.callsTo('GET /api/me')[0]?.headers.get('Authorization')).toBe('Bearer tok-a')
  })

  it('ends the session and goes to Login on a 401', async () => {
    const toLogin = vi.fn()
    installSession(queryClient, toLogin)
    setToken('tok-expired')
    queryClient.setQueryData(['other'], 1)
    api.on('GET /api/me', () => apiError(401, 'unauthenticated', 'log in to continue'))

    await expect(getMe()).rejects.toMatchObject({ code: 'unauthenticated' })

    expect(getToken()).toBeNull()
    expect(queryClient.getQueryData(['other'])).toBeUndefined()
    expect(toLogin).toHaveBeenCalledOnce()
  })

  it('keeps the session on other errors', async () => {
    const toLogin = vi.fn()
    installSession(queryClient, toLogin)
    setToken('tok')
    api.on('GET /api/me', () => apiError(403, 'forbidden', 'no'))

    await expect(getMe()).rejects.toMatchObject({ code: 'forbidden' })

    expect(getToken()).toBe('tok')
    expect(toLogin).not.toHaveBeenCalled()
  })
})
