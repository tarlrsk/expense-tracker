// Small runtime checks for response bodies (ADR-0031): the app checks the fields it depends on
// instead of trusting a cast. Each helper throws ResponseShapeError, which apiRequest turns into
// a `bad_response` ApiError. Extra fields are ignored, so the API can add fields safely.

import { ResponseShapeError } from './client'

export type JsonObject = Record<string, unknown>

export function object(value: unknown, what: string): JsonObject {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ResponseShapeError(`${what} is not an object`)
  }
  return value as JsonObject
}

export function array(value: unknown, what: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new ResponseShapeError(`${what} is not an array`)
  }
  return value
}

export function string(obj: JsonObject, key: string): string {
  const value = obj[key]
  if (typeof value !== 'string') {
    throw new ResponseShapeError(`${key} is not a string`)
  }
  return value
}

export function nonEmptyString(obj: JsonObject, key: string): string {
  const value = string(obj, key)
  if (value === '') {
    throw new ResponseShapeError(`${key} is empty`)
  }
  return value
}

export function boolean(obj: JsonObject, key: string): boolean {
  const value = obj[key]
  if (typeof value !== 'boolean') {
    throw new ResponseShapeError(`${key} is not a boolean`)
  }
  return value
}

/** A timestamp sent as text (RFC 3339); kept as text, checked to be readable. */
export function timestamp(obj: JsonObject, key: string): string {
  const value = string(obj, key)
  if (Number.isNaN(Date.parse(value))) {
    throw new ResponseShapeError(`${key} is not a timestamp`)
  }
  return value
}

export function nullableTimestamp(obj: JsonObject, key: string): string | null {
  return obj[key] === null ? null : timestamp(obj, key)
}

export function oneOf<const T extends string>(
  obj: JsonObject,
  key: string,
  values: readonly T[],
): T {
  const value = string(obj, key)
  if (!(values as readonly string[]).includes(value)) {
    throw new ResponseShapeError(`${key} has an unknown value`)
  }
  return value as T
}

export function nullableString(obj: JsonObject, key: string): string | null {
  return obj[key] === null ? null : string(obj, key)
}

/** A whole number. */
export function integer(obj: JsonObject, key: string): number {
  const value = obj[key]
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) {
    throw new ResponseShapeError(`${key} is not a whole number`)
  }
  return value
}

/** Money as the API writes it, such as "145.00" (ADR-0071); kept as text. */
export function amount(obj: JsonObject, key: string): string {
  const value = string(obj, key)
  if (!/^-?\d+\.\d{2}$/.test(value)) {
    throw new ResponseShapeError(`${key} is not an amount`)
  }
  return value
}

/** A calendar day written YYYY-MM-DD. */
export function date(obj: JsonObject, key: string): string {
  const value = string(obj, key)
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    throw new ResponseShapeError(`${key} is not a date`)
  }
  return value
}
