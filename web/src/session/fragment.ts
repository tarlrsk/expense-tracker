// The set-password link is `…/set-password#token=<token>` (ADR-0068): the part after `#` never
// reaches a server. The screen reads the token once, keeps it only in memory, and removes it
// from the address bar so it is not left in the history, a bookmark or a screenshot.

/** The `token` value of a URL fragment such as `#token=abc`, or null when there is none. */
export function readFragmentToken(hash: string): string | null {
  const fragment = hash.startsWith('#') ? hash.slice(1) : hash
  if (fragment === '') {
    return null
  }
  const token = new URLSearchParams(fragment).get('token')
  return token === null || token === '' ? null : token
}

/**
 * Removes the fragment from the address bar with history.replaceState: no navigation, no new
 * history entry, and the router's own history state is kept.
 */
export function removeFragment(): void {
  const { pathname, search, hash } = window.location
  if (hash === '') {
    return
  }
  window.history.replaceState(window.history.state, '', pathname + search)
}
