import { screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { apiError, json, mockApi, noContent, operatorProfile, profile } from '@/test/api'
import { renderApp } from '@/test/render'

const self = {
  id: operatorProfile.id,
  email: operatorProfile.email,
  display_name: 'Kanya',
  role: 'operator',
  status: 'active',
  created_at: '2026-10-01T03:00:00Z',
  last_active_at: '2026-10-03T07:05:00Z',
}

const invited = {
  id: '0190a1b2-0000-7000-8000-000000000002',
  email: 'somchai@example.com',
  display_name: '',
  role: 'user',
  status: 'invited',
  // 20:30 UTC on the 1st is the 2nd in Bangkok.
  created_at: '2026-10-01T20:30:00Z',
  last_active_at: null,
}

let api: ReturnType<typeof mockApi>
let users: (typeof self | typeof invited)[]

beforeEach(() => {
  users = [self, invited]
  api = mockApi({
    'GET /api/me': () => json(200, operatorProfile),
    'GET /api/admin/users': () => json(200, { users }),
  })
})

function openAdmin() {
  return renderApp('/settings/admin', { token: 'tok' })
}

async function rowOf(email: string) {
  const list = await screen.findByRole('list', { name: 'Admin' })
  const row = within(list)
    .getAllByRole('listitem')
    .find((li) => li.textContent.includes(email))
  if (!row) {
    throw new Error(`no row for ${email}`)
  }
  return row
}

describe('Admin', () => {
  it('lists accounts with their mark, dates in Bangkok time and last active', async () => {
    openAdmin()

    expect(await screen.findByTestId('users-skeleton')).toBeInTheDocument()
    const mine = await rowOf(self.email)
    expect(mine).toHaveTextContent('(You)')
    expect(mine).toHaveTextContent('Kanya')
    expect(mine).toHaveTextContent('Active')
    expect(mine).toHaveTextContent('Joined 1 Oct 2026')
    expect(mine).toHaveTextContent('Last active 3 Oct 2026, 14:05')
    const other = await rowOf(invited.email)
    expect(other).toHaveTextContent('Invited')
    expect(other).toHaveTextContent('Joined 2 Oct 2026')
    expect(other).toHaveTextContent('Not active yet')
  })

  it('offers no Remove on the operator’s own row', async () => {
    openAdmin()

    const mine = await rowOf(self.email)
    expect(within(mine).queryByRole('button', { name: 'Remove' })).toBeNull()
    expect(within(mine).getByRole('button', { name: 'Send new link' })).toBeInTheDocument()
    const other = await rowOf(invited.email)
    expect(within(other).getByRole('button', { name: 'Remove' })).toBeInTheDocument()
  })

  it('invites someone and refreshes the list', async () => {
    const created = {
      ...invited,
      id: '0190a1b2-0000-7000-8000-000000000003',
      email: 'new@example.com',
    }
    api.on('POST /api/admin/invites', () => {
      users = [...users, created]
      return json(201, { user: created, email_sent: true })
    })
    const { user } = openAdmin()

    await user.click(await screen.findByRole('button', { name: 'Invite someone' }))
    const sheet = await screen.findByRole('dialog', { name: 'Invite someone' })
    const email = within(sheet).getByLabelText('Email')
    expect(email).toHaveAttribute('type', 'email')
    expect(email).toHaveAttribute('inputmode', 'email')
    await user.type(email, 'new@example.com')
    await user.click(within(sheet).getByRole('button', { name: 'Send invite' }))

    expect(await screen.findByText('Invite sent to new@example.com.')).toBeInTheDocument()
    expect(await rowOf('new@example.com')).toBeInTheDocument()
    expect(api.callsTo('POST /api/admin/invites')[0]?.body).toEqual({ email: 'new@example.com' })
  })

  it('says plainly when the invite email could not be sent, and offers a new link', async () => {
    const created = {
      ...invited,
      id: '0190a1b2-0000-7000-8000-000000000003',
      email: 'new@example.com',
    }
    api.on('POST /api/admin/invites', () => json(201, { user: created, email_sent: false }))
    api.on(`POST /api/admin/users/${created.id}/set-password-link`, () =>
      json(200, { email_sent: true }),
    )
    const { user } = openAdmin()

    await user.click(await screen.findByRole('button', { name: 'Invite someone' }))
    let sheet = await screen.findByRole('dialog', { name: 'Invite someone' })
    await user.type(within(sheet).getByLabelText('Email'), 'new@example.com')
    await user.click(within(sheet).getByRole('button', { name: 'Send invite' }))

    sheet = await screen.findByRole('dialog', { name: 'Invite someone' })
    expect(
      await within(sheet).findByText(
        'The account for new@example.com was created, but the email could not be sent.',
      ),
    ).toBeInTheDocument()
    await user.click(within(sheet).getByRole('button', { name: 'Send new link' }))

    expect(await screen.findByText('A new link was sent to new@example.com.')).toBeInTheDocument()
    expect(api.callsTo(`POST /api/admin/users/${created.id}/set-password-link`)).toHaveLength(1)
  })

  it('shows an email that already has an account on the field', async () => {
    api.on('POST /api/admin/invites', () =>
      apiError(409, 'conflict', 'this email already has an account'),
    )
    const { user } = openAdmin()

    await user.click(await screen.findByRole('button', { name: 'Invite someone' }))
    const sheet = await screen.findByRole('dialog', { name: 'Invite someone' })
    const email = within(sheet).getByLabelText('Email')
    await user.type(email, invited.email)
    await user.click(within(sheet).getByRole('button', { name: 'Send invite' }))

    await waitFor(() => {
      expect(email).toHaveAccessibleDescription('This email already has an account.')
    })
  })

  it.each([
    { sent: true, text: 'A new link was sent to somchai@example.com.' },
    { sent: false, text: 'the email could not be sent' },
  ])('sends a new link from a row (email_sent: $sent)', async ({ sent, text }) => {
    api.on(`POST /api/admin/users/${invited.id}/set-password-link`, () =>
      json(200, { email_sent: sent }),
    )
    const { user } = openAdmin()

    const row = await rowOf(invited.email)
    await user.click(within(row).getByRole('button', { name: 'Send new link' }))

    expect(await within(row).findByText(new RegExp(text))).toBeInTheDocument()
  })

  it('removes a user after a confirmation that names the email', async () => {
    api.on(`DELETE /api/admin/users/${invited.id}`, () => {
      users = [self]
      return noContent()
    })
    const { user } = openAdmin()

    const row = await rowOf(invited.email)
    await user.click(within(row).getByRole('button', { name: 'Remove' }))
    const sheet = await screen.findByRole('dialog', { name: 'Remove somchai@example.com?' })
    expect(sheet).toHaveTextContent(/all their data/)
    await user.click(within(sheet).getByRole('button', { name: 'Remove' }))

    expect(await screen.findByText('somchai@example.com was removed.')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.getByRole('list', { name: 'Admin' })).not.toHaveTextContent(invited.email)
    })
  })

  it('cancels a removal', async () => {
    const { user } = openAdmin()

    const row = await rowOf(invited.email)
    await user.click(within(row).getByRole('button', { name: 'Remove' }))
    const sheet = await screen.findByRole('dialog', { name: 'Remove somchai@example.com?' })
    await user.click(within(sheet).getByRole('button', { name: 'Cancel' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog')).toBeNull()
    })
    expect(api.callsTo(`DELETE /api/admin/users/${invited.id}`)).toHaveLength(0)
  })

  it('shows the API conflict when a removal is refused', async () => {
    api.on(`DELETE /api/admin/users/${invited.id}`, () =>
      apiError(
        409,
        'conflict',
        'the last operator account cannot be removed; make another account an operator first',
      ),
    )
    const { user } = openAdmin()

    await user.click(within(await rowOf(invited.email)).getByRole('button', { name: 'Remove' }))
    const sheet = await screen.findByRole('dialog', { name: 'Remove somchai@example.com?' })
    await user.click(within(sheet).getByRole('button', { name: 'Remove' }))

    expect(await within(sheet).findByRole('alert')).toHaveTextContent(
      'The last operator account cannot be removed',
    )
  })
})

describe('the operator-only guard', () => {
  it('sends a non-operator back to Settings without asking for the list', async () => {
    api.on('GET /api/me', () => json(200, profile))
    const { router } = openAdmin()

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/settings')
    })
    expect(api.callsTo('GET /api/admin/users')).toHaveLength(0)
  })

  it('handles a 403 from the API the same way', async () => {
    // The cached role says operator, but the API no longer agrees.
    let role = 'operator'
    api.on('GET /api/me', () => json(200, { ...operatorProfile, role }))
    api.on('GET /api/admin/users', () => {
      role = 'user'
      return apiError(403, 'forbidden', 'only an operator may do this')
    })
    const { router } = openAdmin()

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/settings')
    })
    expect(await screen.findByText('kanya@example.com')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Admin/ })).toBeNull()
  })
})
