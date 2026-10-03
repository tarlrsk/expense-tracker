import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, json, mockApi, noContent, profile } from '@/test/api'

import { setTokenProvider } from './client'
import { changePassword, deleteMe, getMe, updateMe } from './me'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

describe('getMe', () => {
  it('gets the profile with the bearer token', async () => {
    api.on('GET /api/me', () => json(200, profile))

    await expect(getMe()).resolves.toEqual(profile)

    expect(api.callsTo('GET /api/me')[0]?.headers.get('Authorization')).toBe('Bearer tok')
  })

  it.each([
    { name: 'an unknown role', body: { ...profile, role: 'admin' } },
    { name: 'no email', body: { ...profile, email: undefined } },
    { name: 'a null display name', body: { ...profile, display_name: null } },
    { name: 'no id', body: { ...profile, id: '' } },
    { name: 'a bad created_at', body: { ...profile, created_at: 'yesterday' } },
  ])('refuses a profile with $name', async ({ body }) => {
    api.on('GET /api/me', () => json(200, body))

    await expect(getMe()).rejects.toMatchObject({ code: 'bad_response', status: 200 })
  })
})

describe('updateMe', () => {
  it('patches only the display name and returns the profile', async () => {
    const updated = { ...profile, display_name: 'Kan' }
    api.on('PATCH /api/me', () => json(200, updated))

    await expect(updateMe({ display_name: 'Kan' })).resolves.toEqual(updated)

    expect(api.callsTo('PATCH /api/me')[0]?.body).toEqual({ display_name: 'Kan' })
  })

  it('passes a rule error through', async () => {
    api.on('PATCH /api/me', () =>
      apiError(400, 'invalid_input', 'the display name must be at most 50 characters long'),
    )

    await expect(updateMe({ display_name: 'x' })).rejects.toMatchObject({ code: 'invalid_input' })
  })
})

describe('deleteMe', () => {
  it('sends the password in a DELETE body', async () => {
    api.on('DELETE /api/me', () => noContent())

    await expect(deleteMe({ password: 'pw' })).resolves.toBeUndefined()

    expect(api.callsTo('DELETE /api/me')[0]?.body).toEqual({ password: 'pw' })
  })

  it.each([
    { status: 400, code: 'invalid_input', message: 'the password is incorrect' },
    {
      status: 409,
      code: 'conflict',
      message:
        'the last operator account cannot be removed; make another account an operator first',
    },
  ])('passes $code through', async ({ status, code, message }) => {
    api.on('DELETE /api/me', () => apiError(status, code, message))

    await expect(deleteMe({ password: 'pw' })).rejects.toMatchObject({ status, code, message })
  })
})

describe('changePassword', () => {
  it('posts both passwords', async () => {
    api.on('POST /api/me/password', () => noContent())

    await expect(
      changePassword({ current_password: 'old-password', new_password: 'new-password' }),
    ).resolves.toBeUndefined()

    expect(api.callsTo('POST /api/me/password')[0]?.body).toEqual({
      current_password: 'old-password',
      new_password: 'new-password',
    })
  })

  it('passes a wrong current password through as invalid_input', async () => {
    api.on('POST /api/me/password', () =>
      apiError(400, 'invalid_input', 'the current password is incorrect'),
    )

    await expect(
      changePassword({ current_password: 'x', new_password: 'new-password' }),
    ).rejects.toMatchObject({ code: 'invalid_input', status: 400 })
  })
})
