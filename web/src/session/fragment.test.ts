import { afterEach, describe, expect, it } from 'vitest'

import { readFragmentToken, removeFragment } from './fragment'

describe('readFragmentToken', () => {
  it.each([
    { hash: '#token=abc123', want: 'abc123' },
    { hash: 'token=abc123', want: 'abc123' },
    { hash: '#token=a%2Bb', want: 'a+b' },
    { hash: '#x=1&token=abc', want: 'abc' },
    { hash: '#token=', want: null },
    { hash: '#tokens=abc', want: null },
    { hash: '#', want: null },
    { hash: '', want: null },
  ])('reads $hash as $want', ({ hash, want }) => {
    expect(readFragmentToken(hash)).toBe(want)
  })
})

describe('removeFragment', () => {
  afterEach(() => {
    window.history.replaceState(null, '', '/')
  })

  it('removes the fragment without a new history entry and keeps path, query and state', () => {
    window.history.pushState({ key: 'router-key' }, '', '/set-password?x=1#token=secret')
    const length = window.history.length

    removeFragment()

    expect(window.location.hash).toBe('')
    expect(window.location.pathname).toBe('/set-password')
    expect(window.location.search).toBe('?x=1')
    expect(window.location.href).not.toContain('secret')
    expect(window.history.length).toBe(length)
    expect(window.history.state).toEqual({ key: 'router-key' })
  })

  it('does nothing without a fragment', () => {
    window.history.replaceState({ a: 1 }, '', '/set-password')

    removeFragment()

    expect(window.location.pathname).toBe('/set-password')
    expect(window.history.state).toEqual({ a: 1 })
  })
})
