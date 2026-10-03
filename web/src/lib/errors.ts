import { ApiError } from '@/api/client'
import { common } from '@/messages/common'

/** "the email or password is incorrect" → "The email or password is incorrect." */
export function sentence(text: string): string {
  const trimmed = text.trim()
  if (trimmed === '') {
    return trimmed
  }
  const first = trimmed.charAt(0).toUpperCase() + trimmed.slice(1)
  return /[.!?…]$/.test(first) ? first : `${first}.`
}

/**
 * The text to show for a failed call. The API's own messages are safe to show (ADR-0046) and
 * are used for its rule errors; a missing response or an unreadable one gets the app's text.
 */
export function errorText(err: unknown): string {
  if (!(err instanceof ApiError)) {
    return common.unexpectedError
  }
  switch (err.code) {
    case 'network':
      return common.networkError
    case 'bad_response':
    case 'internal':
    case 'timeout':
      return common.unexpectedError
    default:
      return err.message.trim() === '' ? common.unexpectedError : sentence(err.message)
  }
}

export function isApiError(err: unknown, code: ApiError['code']): err is ApiError {
  return err instanceof ApiError && err.code === code
}
