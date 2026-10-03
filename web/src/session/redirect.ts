// Where to go after logging in. Only a path inside this app is accepted, so a crafted
// `?redirect=` cannot send the person to another site.

export const homePath = '/history'

/** The redirect target if it is a safe in-app path, else the home path. */
export function safeRedirect(target: string | undefined): string {
  if (
    target === undefined ||
    !target.startsWith('/') ||
    target.startsWith('//') ||
    target.startsWith('/\\') ||
    target === '/login' ||
    target.startsWith('/login?') ||
    target.startsWith('/set-password')
  ) {
    return homePath
  }
  return target
}
