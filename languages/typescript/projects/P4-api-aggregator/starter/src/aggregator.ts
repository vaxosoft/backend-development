import type { Config } from './config.ts';
import type { AggregateResponse } from './types.ts';

/**
 * TODO(P4): fetch every id with `fetchItem`.
 *
 * - at most `config.maxConcurrency` fetches in flight at any moment
 * - `results` and `errors` both follow the order of `ids`
 * - the whole call has `config.overallTimeoutMs`; when it expires, or when `clientSignal`
 *   aborts, outstanding fetches are cancelled, ids that were not started never start,
 *   and unfinished ids are reported as DEADLINE_EXCEEDED
 */
export async function aggregate(
  _ids: number[],
  _config: Config,
  _clientSignal: AbortSignal,
): Promise<AggregateResponse> {
  throw new Error('TODO');
}
