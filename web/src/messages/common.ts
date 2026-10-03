// Every user-facing string lives in these typed message modules, one object per feature, so a
// Thai version can be added later without touching the components (ADR-0072). English only for
// now; sentence case; plain verbs that say what happens.

export const common = {
  appName: 'Satang',
  close: 'Close',
  cancel: 'Cancel',
  back: 'Back',
  tryAgain: 'Try again',
  networkError: 'The server could not be reached. Check your connection and try again.',
  unexpectedError: 'Something went wrong on the server. Try again in a moment.',
  pageNotFound: 'This page does not exist.',
  goHome: 'Go to History',
  errorTitle: 'Something went wrong',
  showPassword: 'Show password',
  hidePassword: 'Hide password',
  passwordRule: 'At least 10 characters.',
  passwordTooShort: 'Use at least 10 characters.',
  passwordTooLong: 'Use at most 128 characters.',
  enterPassword: 'Enter your password.',
} as const
