export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface PageParams {
  limit: number;
  offset: number;
}

export type ParsedParams = { ok: true; value: PageParams } | { ok: false; message: string };

export type RawQuery = Readonly<Record<string, string | string[] | undefined>>;

const DEFAULT_LIMIT = 20;
const MAX_LIMIT = 100;

function readWholeNumber(raw: string | string[] | undefined, fallback: number): number | null {
  if (raw === undefined) return fallback;
  // Number('') is 0 and Number(' 5') is 5, so check the text first.
  if (typeof raw !== 'string' || !/^\d+$/.test(raw)) return null;
  return Number(raw);
}

/**
 * Reads `limit` and `offset` from raw query-string values.
 * - `limit`: default 20, must be an integer >= 1, values above 100 are clamped to 100
 * - `offset`: default 0, must be an integer >= 0
 * - Only plain decimal digits are accepted (no "", " 5", "1.5", "-1", "abc", "1e3", repeated parameters).
 * On failure return `{ ok: false, message }` where the message names the offending parameter.
 */
export function parsePageParams(query: RawQuery): ParsedParams {
  const limit = readWholeNumber(query.limit, DEFAULT_LIMIT);
  if (limit === null || limit < 1) {
    return { ok: false, message: 'limit must be an integer of at least 1' };
  }
  const offset = readWholeNumber(query.offset, 0);
  if (offset === null) return { ok: false, message: 'offset must be a non-negative integer' };
  return { ok: true, value: { limit: Math.min(limit, MAX_LIMIT), offset } };
}

/** Returns one page of `items` plus the total number of items. Must not modify `items`. */
export function paginate<T>(items: readonly T[], params: PageParams): Page<T> {
  return {
    items: items.slice(params.offset, params.offset + params.limit),
    total: items.length,
    limit: params.limit,
    offset: params.offset,
  };
}
