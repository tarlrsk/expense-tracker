export const setPassword = {
  title: 'Set your password',
  intro: 'Choose the password you will log in with.',
  password: 'New password',
  submit: 'Save password',
  pending: 'Saving…',
  noLinkTitle: 'This link does not work',
  // The same explanation the API gives for a bad link (ADR-0066).
  noLink: 'This link is invalid or has expired.',
  askNewLink: 'Ask the person who invited you to send a new link.',
  toLogin: 'Go to log in',
} as const
