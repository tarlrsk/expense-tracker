import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { monthOf, periodLabel, today } from '@/lib/dates'
import { saveFile } from '@/lib/download'
import { getToken } from '@/session/token'
import { apiError, json, mockApi, noContent, operatorProfile, profile } from '@/test/api'
import { renderApp } from '@/test/render'

vi.mock('@/lib/download', () => ({ saveFile: vi.fn() }))

let api: ReturnType<typeof mockApi>

beforeEach(() => {
  api = mockApi({ 'GET /api/me': () => json(200, profile) })
  vi.mocked(saveFile).mockClear()
})

const csvHeader = '\uFEFFDate,Amount,Kind,Category,Description,Note,Source\r\n'

function csv(body: string, fileName: string): Response {
  return new Response(body, {
    status: 200,
    headers: {
      'Content-Type': 'text/csv; charset=utf-8',
      'Content-Disposition': `attachment; filename="${fileName}"`,
    },
  })
}

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

  it('shows the saved description style on its row', async () => {
    api.on('GET /api/me', () => json(200, { ...profile, description_style: 'light_tidy' }))
    openSettings()

    const row = await screen.findByRole('button', { name: /Description style/ })
    expect(row).toHaveTextContent('Light tidy')
  })

  it('saves a description style at once, updates the row and closes the sheet', async () => {
    api.on('PATCH /api/me', () => json(200, { ...profile, description_style: 'clean_name' }))
    const { user } = openSettings()

    const row = await screen.findByRole('button', { name: /Description style/ })
    expect(row).toHaveTextContent('As typed')
    await user.click(row)
    const sheet = await screen.findByRole('dialog', { name: 'Description style' })
    expect(sheet).toHaveTextContent(/Entries your own rules match stay as typed/)
    expect(within(sheet).getByRole('radio', { name: 'As typed' })).toBeChecked()
    expect(within(sheet).getByRole('radio', { name: 'As typed' })).toHaveAccessibleDescription(
      'กาแฟ amazon 60 becomes กาแฟ amazon',
    )
    expect(within(sheet).getByRole('radio', { name: 'Light tidy' })).toHaveAccessibleDescription(
      'กาแฟ amazon 60 becomes กาแฟ Amazon',
    )
    const clean = within(sheet).getByRole('radio', { name: 'Clean name' })
    expect(clean).toHaveAccessibleDescription('กาแฟ amazon 60 becomes Café Amazon')
    await user.click(clean)

    expect(await screen.findByText('Description style saved.')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(api.callsTo('PATCH /api/me')).toHaveLength(1)
    expect(api.callsTo('PATCH /api/me')[0]?.body).toEqual({ description_style: 'clean_name' })
    expect(screen.getByRole('button', { name: /Description style/ })).toHaveTextContent(
      'Clean name',
    )
  })

  it('disables the options while saving', async () => {
    let answer: (r: Response) => void = () => undefined
    api.on(
      'PATCH /api/me',
      () =>
        new Promise<Response>((resolve) => {
          answer = resolve
        }),
    )
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Description style/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Description style' })
    await user.click(within(sheet).getByRole('radio', { name: 'Light tidy' }))

    await waitFor(() => {
      expect(within(sheet).getByRole('radio', { name: 'Light tidy' })).toBeDisabled()
    })
    expect(within(sheet).getByRole('radio', { name: 'Light tidy' })).toBeChecked()
    expect(within(sheet).getByRole('radio', { name: 'As typed' })).toBeDisabled()
    answer(json(200, { ...profile, description_style: 'light_tidy' }))

    expect(await screen.findByText('Description style saved.')).toBeInTheDocument()
  })

  it('keeps the sheet open with the error and the saved choice when saving fails', async () => {
    api.on('PATCH /api/me', () => apiError(500, 'internal', 'something went wrong'))
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Description style/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Description style' })
    await user.click(within(sheet).getByRole('radio', { name: 'Clean name' }))

    expect(await within(sheet).findByRole('alert')).toBeInTheDocument()
    expect(within(sheet).getByRole('radio', { name: 'As typed' })).toBeChecked()
    expect(within(sheet).getByRole('radio', { name: 'Clean name' })).not.toBeChecked()
    expect(within(sheet).getByRole('radio', { name: 'Clean name' })).toBeEnabled()
    expect(screen.getByRole('dialog', { name: 'Description style' })).toBeInTheDocument()
    expect(screen.queryByText('Description style saved.')).toBeNull()
  })

  it('closes without saving when the saved choice is tapped', async () => {
    api.on('GET /api/me', () => json(200, { ...profile, description_style: 'light_tidy' }))
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Description style/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Description style' })
    await user.click(within(sheet).getByRole('radio', { name: 'Light tidy' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(api.callsTo('PATCH /api/me')).toHaveLength(0)
    expect(screen.queryByText('Description style saved.')).toBeNull()
  })

  it('exports all time at once, saves the file and closes the sheet', async () => {
    api.on('GET /api/export.csv', () =>
      csv(`${csvHeader}2026-10-01,145.00,expense,Food,Grab,,manual\r\n`, 'satang-transactions.csv'),
    )
    const { user } = openSettings()

    const row = await screen.findByRole('button', { name: /Export data/ })
    expect(row).toHaveTextContent('A CSV file for Excel or Google Sheets')
    await user.click(row)
    const sheet = await screen.findByRole('dialog', { name: 'Export data' })
    expect(sheet).toHaveTextContent(/opens in Excel and Google Sheets/)
    const allTime = within(sheet).getByRole('button', { name: 'All time' })
    expect(allTime).toHaveAccessibleDescription('Every transaction you have recorded')
    await user.click(allTime)

    expect(await screen.findByText('Downloading satang-transactions.csv.')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    const call = api.onlyCall('GET /api/export.csv')
    expect([...call.query.keys()]).toEqual([])
    expect(call.headers.get('Authorization')).toBe('Bearer tok')
    expect(saveFile).toHaveBeenCalledOnce()
    const [blob, name] = vi.mocked(saveFile).mock.calls[0]
    expect(name).toBe('satang-transactions.csv')
    expect(await blob.text()).toContain('2026-10-01,145.00')
  })

  it('exports this month in Bangkok time, with the month on the option', async () => {
    const month = monthOf(today())
    const name = `satang-transactions-${month.from}-to-${month.to}.csv`
    api.on('GET /api/export.csv', () =>
      csv(`${csvHeader}${month.from},60.00,expense,Food,Coffee,,text\r\n`, name),
    )
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Export data/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Export data' })
    const thisMonth = within(sheet).getByRole('button', { name: 'This month' })
    expect(thisMonth).toHaveAccessibleDescription(periodLabel(month))
    await user.click(thisMonth)

    expect(await screen.findByText(`Downloading ${name}.`)).toBeInTheDocument()
    const call = api.onlyCall('GET /api/export.csv')
    expect(call.query.get('from')).toBe(month.from)
    expect(call.query.get('to')).toBe(month.to)
    expect(saveFile).toHaveBeenCalledWith(expect.any(Blob), name)
  })

  it('still saves an empty file and says it is empty', async () => {
    api.on('GET /api/export.csv', () => csv(csvHeader, 'satang-transactions.csv'))
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Export data/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Export data' })
    await user.click(within(sheet).getByRole('button', { name: 'All time' }))

    expect(
      await screen.findByText(
        'Downloading satang-transactions.csv. It is empty: there are no transactions in that period.',
      ),
    ).toBeInTheDocument()
    expect(saveFile).toHaveBeenCalledOnce()
  })

  it('disables both options and says it is preparing while the file loads', async () => {
    let answer: (r: Response) => void = () => undefined
    api.on(
      'GET /api/export.csv',
      () =>
        new Promise<Response>((resolve) => {
          answer = resolve
        }),
    )
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Export data/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Export data' })
    await user.click(within(sheet).getByRole('button', { name: 'This month' }))

    const thisMonth = within(sheet).getByRole('button', { name: 'This month' })
    await waitFor(() => {
      expect(thisMonth).toHaveAccessibleDescription('Preparing your file…')
    })
    expect(thisMonth).toHaveAttribute('aria-disabled', 'true')
    expect(within(sheet).getByRole('button', { name: 'All time' })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
    await user.click(within(sheet).getByRole('button', { name: 'All time' }))
    expect(api.callsTo('GET /api/export.csv')).toHaveLength(1)
    answer(csv(csvHeader, 'satang-transactions.csv'))

    expect(await screen.findByText(/Downloading satang-transactions.csv/)).toBeInTheDocument()
  })

  it('keeps the sheet open with the error and lets the user try again', async () => {
    api.on('GET /api/export.csv', () => apiError(500, 'internal', 'something went wrong'))
    const { user } = openSettings()

    await user.click(await screen.findByRole('button', { name: /Export data/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Export data' })
    await user.click(within(sheet).getByRole('button', { name: 'All time' }))

    expect(await within(sheet).findByRole('alert')).toHaveTextContent(
      'Something went wrong on the server. Try again in a moment.',
    )
    expect(saveFile).not.toHaveBeenCalled()
    expect(within(sheet).getByRole('button', { name: 'All time' })).not.toHaveAttribute(
      'aria-disabled',
      'true',
    )
    expect(within(sheet).getByRole('button', { name: 'All time' })).toHaveAccessibleDescription(
      'Every transaction you have recorded',
    )

    api.on('GET /api/export.csv', () => csv(csvHeader, 'satang-transactions.csv'))
    await user.click(within(sheet).getByRole('button', { name: 'All time' }))

    expect(await screen.findByText(/Downloading satang-transactions.csv/)).toBeInTheDocument()
    expect(saveFile).toHaveBeenCalledOnce()
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
