export const settings = {
  title: 'Settings',
  email: 'Email',
  displayName: 'Display name',
  displayNameHint: 'Optional, up to 50 characters.',
  displayNameTooLong: 'Use at most 50 characters.',
  saveName: 'Save name',
  savingName: 'Saving…',
  nameSaved: 'Name saved.',
  categories: 'Categories',
  categoriesHint: 'Add, rename, archive and reorder',
  descriptionStyle: 'Description style',
  changePassword: 'Change password',
  admin: 'Admin',
  adminHint: 'Invite people and manage accounts',
  logOut: 'Log out',
  loggingOut: 'Logging out…',
  deleteAccount: 'Delete account',
  loadError: 'Your account could not be loaded.',

  changePasswordTitle: 'Change password',
  changePasswordIntro: 'You stay logged in here. Your other devices are logged out.',
  currentPassword: 'Current password',
  newPassword: 'New password',
  savePassword: 'Save password',
  savingPassword: 'Saving…',
  passwordChanged: 'Password changed. Your other devices were logged out.',

  descriptionStyleTitle: 'Description style',
  descriptionStyleIntro:
    'How the AI writes the description when it reads an entry. Entries your own rules match stay as typed.',
  descriptionStyleSaved: 'Description style saved.',
  /** What the examples start from: the same typed entry for every style. */
  descriptionStyleExampleInput: 'กาแฟ amazon 60',
  descriptionStyleExampleBecomes: 'becomes',
  descriptionStyles: {
    as_typed: { label: 'As typed', example: 'กาแฟ amazon' },
    light_tidy: { label: 'Light tidy', example: 'กาแฟ Amazon' },
    clean_name: { label: 'Clean name', example: 'Café Amazon' },
  },

  exportData: 'Export data',
  exportDataHint: 'A CSV file for Excel or Google Sheets',
  exportTitle: 'Export data',
  exportIntro:
    'Download your transactions as a CSV file. It opens in Excel and Google Sheets, Thai text included.',
  exportAllTime: 'All time',
  exportAllTimeHint: 'Every transaction you have recorded',
  exportThisMonth: 'This month',
  exportPreparing: 'Preparing your file…',
  exportStarted: (fileName: string) => `Downloading ${fileName}.`,
  exportStartedEmpty: (fileName: string) =>
    `Downloading ${fileName}. It is empty: there are no transactions in that period.`,

  deleteTitle: 'Delete your account?',
  deleteWarning:
    'This permanently deletes your account and all your data: every expense, income and category. It cannot be undone.',
  deletePassword: 'Your password',
  deleteSubmit: 'Delete account',
  deleting: 'Deleting…',
} as const
