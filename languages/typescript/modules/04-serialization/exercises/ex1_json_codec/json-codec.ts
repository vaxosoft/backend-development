export type DecodeResult = { ok: true; value: unknown } | { ok: false; error: string };

/**
 * JSON.stringify that never throws on bigint and understands Set and Map:
 * bigint -> decimal string, Set -> array, Map -> plain object (keys via String()).
 */
export function encodeJson(_value: unknown): string {
  throw new Error('TODO');
}

/** Parses JSON without throwing: malformed input yields `{ ok: false, error }` with a non-empty message. */
export function decodeJson(_text: string): DecodeResult {
  throw new Error('TODO');
}

/** Like decodeJson, but every property whose name ends in `_at` and holds a string becomes a Date. */
export function decodeWithDates(_text: string): DecodeResult {
  throw new Error('TODO');
}
