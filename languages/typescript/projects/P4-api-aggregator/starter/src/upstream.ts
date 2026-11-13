import type { Config } from './config.ts';
import type { FetchOutcome } from './types.ts';

/**
 * TODO(P4): fetch `GET {upstreamBaseUrl}/items/{id}` and describe what happened.
 *
 * - every attempt gets its own timeout (`config.callTimeoutMs`) and also honours `signal`
 * - retry up to `config.maxRetries` times on network error, timeout and 5xx;
 *   the delay before retry n (0-based) is `config.backoffBaseMs * 2 ** n`
 * - never retry 4xx or an unparsable 2xx body
 * - when `signal` aborts, stop at once and return DEADLINE_EXCEEDED
 * - error texts: `upstream status 500`, `timeout`, `upstream unreachable`,
 *   `invalid upstream response`, `deadline exceeded`
 * - never throw for an expected failure; return `{ ok: false, error }`
 */
export async function fetchItem(
  _config: Config,
  _id: number,
  _signal: AbortSignal,
): Promise<FetchOutcome> {
  throw new Error('TODO');
}
