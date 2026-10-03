import { common } from '@/messages/common'

// The API's rule (ADR-0066): 10 to 128 characters, counted as Go counts runes, so the check
// here and the API's agree; no composition rules.
export const minPasswordLength = 10
export const maxPasswordLength = 128

/** Counts Unicode code points, as the API does, not UTF-16 units. */
export function characterCount(text: string): number {
  return Array.from(text).length
}

/** The message for a new password that breaks the rule, or null when it is fine. */
export function passwordRuleError(password: string): string | null {
  const n = characterCount(password)
  if (n < minPasswordLength) {
    return common.passwordTooShort
  }
  if (n > maxPasswordLength) {
    return common.passwordTooLong
  }
  return null
}
