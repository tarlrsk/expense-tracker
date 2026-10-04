// Transaction ids are made by the app (ADR-0040): a UUID version 7 (RFC 9562) in the usual
// 36-character form, which the API requires (ADR-0071). A small function of our own, not a
// package (ADR-0074).

/** Fills `bytes` with random values; `crypto.getRandomValues` outside tests. */
export type RandomFill = (bytes: Uint8Array<ArrayBuffer>) => void

const defaultFill: RandomFill = (bytes) => {
  crypto.getRandomValues(bytes)
}

/**
 * A new UUID version 7: 48 bits of Unix time in milliseconds, then the version, 12 random bits,
 * the variant and 62 random bits. Ids made later sort after earlier ones (to the millisecond).
 */
export function uuidv7(now: number = Date.now(), fill: RandomFill = defaultFill): string {
  const bytes = new Uint8Array(16)
  fill(bytes)
  // The time fits in 48 bits until the year 10889; Numbers are exact up to 2^53.
  let time = Math.max(0, Math.floor(now))
  for (let i = 5; i >= 0; i--) {
    bytes[i] = time % 256
    time = Math.floor(time / 256)
  }
  bytes[6] = 0x70 | (bytes[6] & 0x0f)
  bytes[8] = 0x80 | (bytes[8] & 0x3f)
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20, 32),
  ].join('-')
}
