export const admin = {
  title: 'Admin',
  intro: 'People with an account. You never see their expenses.',
  invite: 'Invite someone',
  invited: 'Invited',
  active: 'Active',
  you: 'You',
  joined: (date: string) => `Joined ${date}`,
  lastActive: (when: string) => `Last active ${when}`,
  neverActive: 'Not active yet',
  sendLink: 'Send new link',
  sendingLink: 'Sending…',
  remove: 'Remove',
  loadError: 'The list of accounts could not be loaded.',
  empty: 'No accounts yet.',

  linkSent: (email: string) => `A new link was sent to ${email}.`,
  linkNotSent: (email: string) =>
    `A new link was made for ${email}, but the email could not be sent. Try again later.`,

  inviteTitle: 'Invite someone',
  inviteIntro:
    'They get an email with a link to set their password. The link works once, for 7 days.',
  inviteEmail: 'Email',
  inviteSubmit: 'Send invite',
  inviting: 'Sending…',
  invitedOk: (email: string) => `Invite sent to ${email}.`,
  inviteNotSent: (email: string) =>
    `The account for ${email} was created, but the email could not be sent.`,
  inviteNotSentHint: 'Send a new link to try the email again.',
  enterEmail: 'Enter an email address.',

  removeTitle: (email: string) => `Remove ${email}?`,
  removeWarning:
    'This deletes their account and all their data: every expense, income and category. It cannot be undone.',
  removeSubmit: 'Remove',
  removing: 'Removing…',
  removed: (email: string) => `${email} was removed.`,
} as const
