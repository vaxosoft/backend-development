export type DecodeResult = { ok: true; value: unknown } | { ok: false; error: string };

/**
 * JSON.stringify that never throws on bigint and understands Set and Map:
 * bigint -> decimal string, Set -> array, Map -> plain object (keys via String()).
 */
export function encodeJson(value: unknown): string {
  return JSON.stringify(value, (_key, v: unknown) => {
    if (typeof v === 'bigint') return v.toString();
    if (v instanceof Set) return [...v];
    if (v instanceof Map) return Object.fromEntries([...v].map(([k, item]) => [String(k), item]));
    return v;
  });
}

/** Parses JSON without throwing: malformed input yields `{ ok: false, error }` with a non-empty message. */
export function decodeJson(text: string): DecodeResult {
  try {
    return { ok: true, value: JSON.parse(text) };
  } catch (err: unknown) {
    if (err instanceof SyntaxError) return { ok: false, error: err.message };
    throw err;
  }
}

/** Like decodeJson, but every property whose name ends in `_at` and holds a string becomes a Date. */
export function decodeWithDates(text: string): DecodeResult {
  try {
    return {
      ok: true,
      value: JSON.parse(text, (key, v: unknown) =>
        key.endsWith('_at') && typeof v === 'string' ? new Date(v) : v,
      ),
    };
  } catch (err: unknown) {
    if (err instanceof SyntaxError) return { ok: false, error: err.message };
    throw err;
  }
}
