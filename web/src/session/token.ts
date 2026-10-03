// The session token (ADR-0037): one localStorage key, so the installed app stays logged in.
// Any script on the page can read it, so the app loads no third-party script, font host or
// analytics. The token is never logged and never put in a URL.

/** The one localStorage key the token is kept under. */
export const tokenStorageKey = 'satang.session-token'

/** The stored token, or null when logged out or when storage cannot be read. */
export function getToken(): string | null {
  try {
    const token = localStorage.getItem(tokenStorageKey)
    return token === '' ? null : token
  } catch {
    return null
  }
}

export function setToken(token: string): void {
  localStorage.setItem(tokenStorageKey, token)
}

export function clearToken(): void {
  try {
    localStorage.removeItem(tokenStorageKey)
  } catch {
    // Storage is unavailable, so nothing is stored either.
  }
}
