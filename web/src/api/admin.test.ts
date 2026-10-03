import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { apiError, json, mockApi, noContent } from '@/test/api'

import { inviteUser, listUsers, removeUser, sendSetPasswordLink } from './admin'
import { setTokenProvider } from './client'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi()
  setTokenProvider(() => 'tok')
})

afterEach(() => {
  setTokenProvider(null)
})

const user = {
  id: '0190a1b2-0000-7000-8000-000000000002',
  email: 'somchai@example.com',
  display_name: '',
  role: 'user',
  status: 'invited',
  created_at: '2026-10-02T03:00:00Z',
  last_active_at: null,
}

describe('listUsers', () => {
  it('gets the list', async () => {
    const active = { ...user, status: 'active', last_active_at: '2026-10-03T07:05:00Z' }
    api.on('GET /api/admin/users', () => json(200, { users: [user, active] }))

    await expect(listUsers()).resolves.toEqual({ users: [user, active] })
  })

  it.each([
    { name: 'no users key', body: {} },
    { name: 'an unknown status', body: { users: [{ ...user, status: 'disabled' }] } },
    { name: 'a missing last_active_at', body: { users: [{ ...user, last_active_at: undefined }] } },
    { name: 'a bad last_active_at', body: { users: [{ ...user, last_active_at: 'never' }] } },
    { name: 'a user that is not an object', body: { users: ['x'] } },
  ])('refuses a list with $name', async ({ body }) => {
    api.on('GET /api/admin/users', () => json(200, body))

    await expect(listUsers()).rejects.toMatchObject({ code: 'bad_response' })
  })

  it('passes forbidden through', async () => {
    api.on('GET /api/admin/users', () => apiError(403, 'forbidden', 'only an operator may do this'))

    await expect(listUsers()).rejects.toMatchObject({ code: 'forbidden', status: 403 })
  })
})

describe('inviteUser', () => {
  it('posts the email and returns the user and email_sent', async () => {
    api.on('POST /api/admin/invites', () => json(201, { user, email_sent: false }))

    await expect(inviteUser({ email: user.email })).resolves.toEqual({ user, email_sent: false })

    expect(api.callsTo('POST /api/admin/invites')[0]?.body).toEqual({ email: user.email })
  })

  it('refuses an answer without email_sent', async () => {
    api.on('POST /api/admin/invites', () => json(201, { user }))

    await expect(inviteUser({ email: user.email })).rejects.toMatchObject({ code: 'bad_response' })
  })

  it('passes conflict through', async () => {
    api.on('POST /api/admin/invites', () =>
      apiError(409, 'conflict', 'this email already has an account'),
    )

    await expect(inviteUser({ email: user.email })).rejects.toMatchObject({ code: 'conflict' })
  })
})

describe('sendSetPasswordLink', () => {
  it('posts to the user path, escaped, and returns email_sent', async () => {
    api.on(`POST /api/admin/users/${user.id}/set-password-link`, () =>
      json(200, { email_sent: true }),
    )

    await expect(sendSetPasswordLink(user.id)).resolves.toEqual({ email_sent: true })

    await sendSetPasswordLink('a/b').catch(() => undefined)
    expect(api.calls.at(-1)?.path).toBe('/api/admin/users/a%2Fb/set-password-link')
  })

  it('refuses an answer without email_sent', async () => {
    api.on(`POST /api/admin/users/${user.id}/set-password-link`, () => json(200, {}))

    await expect(sendSetPasswordLink(user.id)).rejects.toMatchObject({ code: 'bad_response' })
  })
})

describe('removeUser', () => {
  it('deletes the user path', async () => {
    api.on(`DELETE /api/admin/users/${user.id}`, () => noContent())

    await expect(removeUser(user.id)).resolves.toBeUndefined()
  })

  it('passes conflict through', async () => {
    api.on(`DELETE /api/admin/users/${user.id}`, () =>
      apiError(
        409,
        'conflict',
        'you cannot remove your own account here; delete it from Settings instead',
      ),
    )

    await expect(removeUser(user.id)).rejects.toMatchObject({ code: 'conflict', status: 409 })
  })
})
