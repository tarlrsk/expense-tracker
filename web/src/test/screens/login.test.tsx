import { screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { common } from '@/messages/common'
import { getToken } from '@/session/token'
import { apiError, json, mockApi, profile, session } from '@/test/api'
import { renderApp } from '@/test/render'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi({ 'GET /api/me': () => json(200, profile) })
})

async function fillIn(user: ReturnType<typeof renderApp>['user']) {
  await user.type(await screen.findByLabelText('Email'), 'kanya@example.com')
  await user.type(screen.getByLabelText('Password'), 'a-good-password')
  await user.click(screen.getByRole('button', { name: 'Log in' }))
}

describe('Login', () => {
  it('has the right keyboards and autofill', async () => {
    renderApp('/login')

    const email = await screen.findByLabelText('Email')
    expect(email).toHaveAttribute('type', 'email')
    expect(email).toHaveAttribute('inputmode', 'email')
    expect(email).toHaveAttribute('autocomplete', 'username')
    const password = screen.getByLabelText('Password')
    expect(password).toHaveAttribute('type', 'password')
    expect(password).toHaveAttribute('autocomplete', 'current-password')
    expect(screen.queryByRole('navigation')).toBeNull()
  })

  it('logs in, stores the token and opens the app', async () => {
    api.on('POST /api/auth/login', () => json(200, session))
    const { user, router } = renderApp('/login')

    await fillIn(user)

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/history')
    })
    expect(getToken()).toBe(session.token)
    expect(api.callsTo('POST /api/auth/login')[0]?.body).toEqual({
      email: 'kanya@example.com',
      password: 'a-good-password',
    })
    expect(await screen.findByRole('navigation', { name: 'Main' })).toBeInTheDocument()
  })

  it('shows the API message for a wrong email or password', async () => {
    api.on('POST /api/auth/login', () =>
      apiError(401, 'unauthenticated', 'the email or password is incorrect'),
    )
    const { user, router } = renderApp('/login')

    await fillIn(user)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The email or password is incorrect.',
    )
    expect(getToken()).toBeNull()
    expect(router.state.location.pathname).toBe('/login')
  })

  it('says to wait when rate limited', async () => {
    api.on('POST /api/auth/login', () =>
      apiError(429, 'rate_limited', 'too many failed attempts; try again in 15 minutes'),
    )
    const { user } = renderApp('/login')

    await fillIn(user)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Too many failed attempts; try again in 15 minutes.',
    )
  })

  it('says the server could not be reached', async () => {
    api.on('POST /api/auth/login', () => Promise.reject(new TypeError('Failed to fetch')))
    const { user } = renderApp('/login')

    await fillIn(user)

    expect(await screen.findByRole('alert')).toHaveTextContent(common.networkError)
  })

  it('asks for both fields before calling the API', async () => {
    const { user } = renderApp('/login')

    await user.click(await screen.findByRole('button', { name: 'Log in' }))

    expect(screen.getByLabelText('Email')).toHaveAccessibleDescription('Enter your email.')
    expect(screen.getByLabelText('Password')).toHaveAccessibleDescription('Enter your password.')
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true')
    expect(api.callsTo('POST /api/auth/login')).toHaveLength(0)
  })

  it('shows the pending state on the button', async () => {
    let answer: (r: Response) => void = () => undefined
    api.on('POST /api/auth/login', () => new Promise<Response>((resolve) => (answer = resolve)))
    const { user } = renderApp('/login')

    await fillIn(user)

    // Disabled but still focusable, so a screen reader keeps its place.
    expect(await screen.findByRole('button', { name: 'Logging in…' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    answer(json(200, session))
  })

  it('shows and hides the password', async () => {
    const { user } = renderApp('/login')
    const password = await screen.findByLabelText('Password')

    await user.click(screen.getByRole('button', { name: 'Show password' }))
    expect(password).toHaveAttribute('type', 'text')
    await user.click(screen.getByRole('button', { name: 'Hide password' }))
    expect(password).toHaveAttribute('type', 'password')
  })
})

describe('route guards', () => {
  it('sends a logged-out visitor to Login and back to where they were going', async () => {
    api.on('POST /api/auth/login', () => json(200, session))
    const { user, router } = renderApp('/settings')

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/login')
    })
    expect(router.state.location.search).toEqual({ redirect: '/settings' })

    await fillIn(user)

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/settings')
    })
  })

  it('sends a logged-in visitor away from Login', async () => {
    const { router } = renderApp('/login', { token: 'tok' })

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/history')
    })
  })

  it('does not follow a redirect to another site', async () => {
    api.on('POST /api/auth/login', () => json(200, session))
    const { user, router } = renderApp('/login?redirect=%2F%2Fevil.example')

    await fillIn(user)

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/history')
    })
  })

  it('opens / on History', async () => {
    const { router } = renderApp('/', { token: 'tok' })

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/history')
    })
    expect(await screen.findByRole('heading', { name: 'History' })).toBeInTheDocument()
  })

  it('ends the session on a 401 and goes to Login, then comes back', async () => {
    api.on('GET /api/me', () => apiError(401, 'unauthenticated', 'log in to continue'))
    const { router } = renderApp('/settings', { token: 'tok-expired' })

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/login')
    })
    expect(getToken()).toBeNull()
    expect(router.state.location.search).toEqual({ redirect: '/settings' })
  })
})
