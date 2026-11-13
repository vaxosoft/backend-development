export interface ItemResult {
  id: number;
  data: unknown;
}

export interface ItemError {
  id: number;
  error: string;
}

export interface AggregateResponse {
  results: ItemResult[];
  errors: ItemError[];
}

/** What fetching one id produced. Expected failures are values, not exceptions. */
export type FetchOutcome = { ok: true; data: unknown } | { ok: false; error: string };

export const DEADLINE_EXCEEDED: FetchOutcome = { ok: false, error: 'deadline exceeded' };
