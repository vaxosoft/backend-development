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

/**
 * Reads `limit` and `offset` from raw query-string values.
 * - `limit`: default 20, must be an integer >= 1, values above 100 are clamped to 100
 * - `offset`: default 0, must be an integer >= 0
 * - Only plain decimal digits are accepted (no "", " 5", "1.5", "-1", "abc", "1e3", repeated parameters).
 * On failure return `{ ok: false, message }` where the message names the offending parameter.
 */
export function parsePageParams(_query: RawQuery): ParsedParams {
  throw new Error('TODO');
}

/** Returns one page of `items` plus the total number of items. Must not modify `items`. */
export function paginate<T>(_items: readonly T[], _params: PageParams): Page<T> {
  throw new Error('TODO');
}
