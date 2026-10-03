import { describe, expect, it } from 'vitest'

import { ApiError } from '@/api/client'
import { common } from '@/messages/common'

import { errorText, sentence } from './errors'

describe('sentence', () => {
  it.each([
    { text: 'the email or password is incorrect', want: 'The email or password is incorrect.' },
    {
      text: 'too many failed attempts; try again in 15 minutes',
      want: 'Too many failed attempts; try again in 15 minutes.',
    },
    { text: 'Already a sentence.', want: 'Already a sentence.' },
    { text: '  ', want: '' },
  ])('$text', ({ text, want }) => {
    expect(sentence(text)).toBe(want)
  })
})

describe('errorText', () => {
  it.each([
    {
      err: new ApiError(401, 'unauthenticated', 'the email or password is incorrect'),
      want: 'The email or password is incorrect.',
    },
    {
      err: new ApiError(409, 'conflict', 'this email already has an account'),
      want: 'This email already has an account.',
    },
    { err: new ApiError(0, 'network', 'Could not reach the server.'), want: common.networkError },
    { err: new ApiError(200, 'bad_response', 'x'), want: common.unexpectedError },
    { err: new ApiError(500, 'internal', 'internal error'), want: common.unexpectedError },
    { err: new ApiError(504, 'timeout', 'timeout'), want: common.unexpectedError },
    { err: new Error('boom'), want: common.unexpectedError },
  ])('$err.message', ({ err, want }) => {
    expect(errorText(err)).toBe(want)
  })
})
