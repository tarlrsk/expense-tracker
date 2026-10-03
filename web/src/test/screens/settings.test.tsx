import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { getToken } from '@/session/token'
import { apiError, json, mockApi, noContent, operatorProfile, profile } from '@/test/api'
import { renderApp } from '@/test/render'

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi({ 'GET /api/me': () => json(200, profile) })
})

function openSettings() {
  return renderApp('/settings', { token: 'tok' })
}

describe('Settings', () => {
  it('shows placeholders while loading, then the account', async () => {
    openSettings()

    expect(await screen.findByTestId('settings-skeleton')).toBeInTheDocument()
    expect(await screen.findByText('kanya@example.com')).toBeInTheDocument()
    expect(screen.getByLabelText('Display name')).toHaveValue('Kanya')
    expect(screen.queryByTestId('settings-skeleton')).toBeNull()
  })

  it('shows the app bar with its three places', async () => {
    openSettings()

    const bar = await screen.findByRole('navigation', { name: 'Main' })
    expect(within(bar).getByRole('link', { name: 'History' })).toHaveAttribute('href', '/history')
    expect(within(bar).getByRole('link', { name: 'Add' })).toHaveAttribute('href', '/add')
    expect(within(bar).getByRole('link', { name: 'Settings' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  it('saves the display name', async () => {
    api.on('PATCH /api/me', () => json(200, { ...profile, display_name: 'Kan' }))
    const { user } = openSettings()

    const field = await screen.findByLabelText('Display name')
    await user.clear(field)
    await user.type(field, 'Kan')
    await user.click(screen.getByRole('button', { name: 'Save name' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Name saved.')
    expect(api.callsTo('PATCH /api/me')[0]?.body).toEqual({ display_name: 'Kan' })
    expect(field).toHaveValue('Kan')
  })

  it('shows a display name rule error on the field', async () => {
    api.on('PATCH /api/me', () =>
      apiError(400, 'invalid_input', 'the display name must be at most 50 characters long'),
    )
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Save name' }))

    expect(screen.getByLabelText('Display name')).toHaveAccessibleDescription(
      expect.stringContaining('The display name must be at most 50 characters long.'),
    )
  })

  it('changes the password and says other devices were logged out', async () => {
    api.on('POST /api/me/password', () => noContent())
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Change password' }))
    const sheet = await screen.findByRole('dialog', { name: 'Change password' })
    await user.type(within(sheet).getByLabelText('Current password'), 'old-password')
    await user.type(within(sheet).getByLabelText('New password'), 'new-password-1')
    await user.click(within(sheet).getByRole('button', { name: 'Save password' }))

    expect(await screen.findByText(/Your other devices were logged out/)).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(api.callsTo('POST /api/me/password')[0]?.body).toEqual({
      current_password: 'old-password',
      new_password: 'new-password-1',
    })
    expect(getToken()).toBe('tok')
  })

  it('shows a wrong current password on that field', async () => {
    api.on('POST /api/me/password', () =>
      apiError(400, 'invalid_input', 'the current password is incorrect'),
    )
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Change password' }))
    const sheet = await screen.findByRole('dialog', { name: 'Change password' })
    const current = within(sheet).getByLabelText('Current password')
    expect(current).toHaveAttribute('autocomplete', 'current-password')
    expect(within(sheet).getByLabelText('New password')).toHaveAttribute(
      'autocomplete',
      'new-password',
    )
    await user.type(current, 'wrong-password')
    await user.type(within(sheet).getByLabelText('New password'), 'new-password-1')
    await user.click(within(sheet).getByRole('button', { name: 'Save password' }))

    await waitFor(() => {
      expect(current).toHaveAccessibleDescription('The current password is incorrect.')
    })
    expect(current).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('dialog', { name: 'Change password' })).toBeInTheDocument()
    // A 400 here is not "logged out".
    expect(getToken()).toBe('tok')
  })

  it('logs out: calls the API, forgets the token and opens Login', async () => {
    api.on('POST /api/auth/logout', () => noContent())
    const { user, router } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Log out' }))

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/login')
    })
    expect(api.callsTo('POST /api/auth/logout')).toHaveLength(1)
    expect(getToken()).toBeNull()
  })

  it('logs out here even when the server cannot be reached', async () => {
    api.on('POST /api/auth/logout', () => Promise.reject(new TypeError('offline')))
    const { user, router } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Log out' }))

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/login')
    })
    expect(getToken()).toBeNull()
  })

  it('deletes the account after a warning and the password', async () => {
    api.on('DELETE /api/me', () => noContent())
    const { user, router } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Delete account' }))
    const sheet = await screen.findByRole('dialog', { name: 'Delete your account?' })
    expect(sheet).toHaveTextContent(/permanently deletes your account and all your data/)
    await user.type(within(sheet).getByLabelText('Your password'), 'my-password')
    await user.click(within(sheet).getByRole('button', { name: 'Delete account' }))

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/login')
    })
    expect(api.callsTo('DELETE /api/me')[0]?.body).toEqual({ password: 'my-password' })
    expect(getToken()).toBeNull()
  })

  it('shows the last-operator conflict and keeps the account', async () => {
    api.on('GET /api/me', () => json(200, operatorProfile))
    api.on('DELETE /api/me', () =>
      apiError(
        409,
        'conflict',
        'the last operator account cannot be removed; make another account an operator first',
      ),
    )
    const { user, router } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Delete account' }))
    const sheet = await screen.findByRole('dialog', { name: 'Delete your account?' })
    await user.type(within(sheet).getByLabelText('Your password'), 'my-password')
    await user.click(within(sheet).getByRole('button', { name: 'Delete account' }))

    expect(await within(sheet).findByRole('alert')).toHaveTextContent(
      'The last operator account cannot be removed; make another account an operator first.',
    )
    expect(router.state.location.pathname).toBe('/settings')
    expect(getToken()).toBe('tok')
  })

  it('shows a wrong password on the delete field', async () => {
    api.on('DELETE /api/me', () => apiError(400, 'invalid_input', 'the password is incorrect'))
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: 'Delete account' }))
    const sheet = await screen.findByRole('dialog', { name: 'Delete your account?' })
    const field = within(sheet).getByLabelText('Your password')
    await user.type(field, 'nope')
    await user.click(within(sheet).getByRole('button', { name: 'Delete account' }))

    await waitFor(() => {
      expect(field).toHaveAccessibleDescription('The password is incorrect.')
    })
  })

  it.each([
    { name: 'a user', me: profile, admin: false },
    { name: 'an operator', me: operatorProfile, admin: true },
  ])('shows the Admin entry only to operators: $name', async ({ me, admin }) => {
    api.on('GET /api/me', () => json(200, me))
    openSettings()

    await screen.findByText('kanya@example.com')
    expect(screen.queryByRole('link', { name: /Admin/ }) !== null).toBe(admin)
  })
})
