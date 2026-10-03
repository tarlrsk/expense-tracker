import { screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { getToken } from '@/session/token'
import { apiError, json, mockApi, profile, session } from '@/test/api'
import { renderApp } from '@/test/render'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi({ 'GET /api/me': () => json(200, profile) })
})

afterEach(() => {
  window.history.replaceState(null, '', '/')
})

/** Opens the emailed link: the token is in the real address bar's fragment. */
function openLink(fragment: string) {
  window.history.replaceState(null, '', `/set-password${fragment}`)
  return renderApp('/set-password')
}

describe('Set password', () => {
  it('reads the token from the fragment and removes it from the address bar', async () => {
    openLink('#token=link-secret')

    expect(await screen.findByRole('heading', { name: 'Set your password' })).toBeInTheDocument()
    expect(window.location.hash).toBe('')
    expect(window.location.href).not.toContain('link-secret')
    expect(window.location.pathname).toBe('/set-password')
    expect(JSON.stringify(localStorage)).not.toContain('link-secret')
  })

  it('states the rule up front and has new-password autofill', async () => {
    openLink('#token=link-secret')

    const field = await screen.findByLabelText('New password')
    expect(field).toHaveAttribute('autocomplete', 'new-password')
    expect(field).toHaveAccessibleDescription('At least 10 characters.')
  })

  it('saves the password, stores the session and opens the app', async () => {
    api.on('POST /api/auth/set-password', () => json(200, session))
    const { user, router } = openLink('#token=link-secret')

    await user.type(await screen.findByLabelText('New password'), 'a-long-password')
    await user.click(screen.getByRole('button', { name: 'Save password' }))

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/history')
    })
    const call = api.onlyCall('POST /api/auth/set-password')
    expect(call.body).toEqual({ token: 'link-secret', password: 'a-long-password' })
    expect(call.headers.has('Authorization')).toBe(false)
    expect(getToken()).toBe(session.token)
    // The link token went only into that call's body.
    expect(api.calls.filter((c) => JSON.stringify(c).includes('link-secret'))).toHaveLength(1)
  })

  it('checks the length before calling the API', async () => {
    const { user } = openLink('#token=link-secret')

    await user.type(await screen.findByLabelText('New password'), 'short')
    await user.click(screen.getByRole('button', { name: 'Save password' }))

    expect(screen.getByLabelText('New password')).toHaveAccessibleDescription(
      'At least 10 characters. Use at least 10 characters.',
    )
    expect(api.callsTo('POST /api/auth/set-password')).toHaveLength(0)
  })

  it('explains an invalid or expired link and says to ask for a new one', async () => {
    api.on('POST /api/auth/set-password', () =>
      apiError(400, 'invalid_input', 'this link is invalid or has expired'),
    )
    const { user } = openLink('#token=old-link')

    await user.type(await screen.findByLabelText('New password'), 'a-long-password')
    await user.click(screen.getByRole('button', { name: 'Save password' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This link is invalid or has expired.')
    expect(alert).toHaveTextContent('Ask the person who invited you to send a new link.')
    expect(screen.queryByLabelText('New password')).toBeNull()
    expect(getToken()).toBeNull()
  })

  it('keeps the form when the server cannot be reached', async () => {
    api.on('POST /api/auth/set-password', () => Promise.reject(new TypeError('offline')))
    const { user } = openLink('#token=link-secret')

    await user.type(await screen.findByLabelText('New password'), 'a-long-password')
    await user.click(screen.getByRole('button', { name: 'Save password' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('could not be reached')
    expect(screen.getByLabelText('New password')).toBeInTheDocument()
  })

  it.each([
    { name: 'no fragment', fragment: '' },
    { name: 'an empty token', fragment: '#token=' },
  ])('shows the same explanation with $name', async ({ fragment }) => {
    openLink(fragment)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This link is invalid or has expired.')
    expect(alert).toHaveTextContent('Ask the person who invited you to send a new link.')
    expect(screen.getByRole('link', { name: 'Go to log in' })).toBeInTheDocument()
    expect(api.calls).toHaveLength(0)
  })
})
