import { describe, expect, it } from 'vitest'

import { homePath, safeRedirect } from './redirect'

describe('safeRedirect', () => {
  it.each([
    { target: undefined, want: homePath },
    { target: '/settings', want: '/settings' },
    { target: '/settings/admin?x=1#y', want: '/settings/admin?x=1#y' },
    { target: 'https://evil.example', want: homePath },
    { target: '//evil.example/path', want: homePath },
    { target: '/\\evil.example', want: homePath },
    { target: 'settings', want: homePath },
    { target: '/login', want: homePath },
    { target: '/login?redirect=/x', want: homePath },
    { target: '/set-password', want: homePath },
  ])('$target → $want', ({ target, want }) => {
    expect(safeRedirect(target)).toBe(want)
  })
})
