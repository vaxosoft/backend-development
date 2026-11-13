import { setTimeout as sleep } from 'node:timers/promises';
import type { Config } from './config.ts';
import { DEADLINE_EXCEEDED, type FetchOutcome } from './types.ts';

interface Attempt {
  outcome: FetchOutcome;
  retry: boolean;
}

function failed(error: string, retry: boolean): Attempt {
  return { outcome: { ok: false, error }, retry };
}

async function attemptOnce(config: Config, id: number, signal: AbortSignal): Promise<Attempt> {
  const url = `${config.upstreamBaseUrl.replace(/\/+$/, '')}/items/${id}`;
  // One fresh timeout per attempt, combined with the caller's cancellation.
  const attemptSignal = AbortSignal.any([signal, AbortSignal.timeout(config.callTimeoutMs)]);
  try {
    const response = await fetch(url, { signal: attemptSignal });
    if (response.status >= 500 || !response.ok) {
      await response.body?.cancel();
      return failed(`upstream status ${response.status}`, response.status >= 500);
    }
    return { outcome: { ok: true, data: await response.json() }, retry: false };
  } catch (error) {
    if (signal.aborted) {
      return { outcome: DEADLINE_EXCEEDED, retry: false };
    }
    if (error instanceof SyntaxError) {
      return failed('invalid upstream response', false);
    }
    if (error instanceof Error && error.name === 'TimeoutError') {
      return failed('timeout', true);
    }
    return failed('upstream unreachable', true);
  }
}

export async function fetchItem(
  config: Config,
  id: number,
  signal: AbortSignal,
): Promise<FetchOutcome> {
  let last: FetchOutcome = DEADLINE_EXCEEDED;
  for (let attempt = 0; attempt <= config.maxRetries; attempt++) {
    try {
      if (attempt > 0) {
        await sleep(config.backoffBaseMs * 2 ** (attempt - 1), undefined, { signal });
      }
    } catch {
      return DEADLINE_EXCEEDED;
    }
    const { outcome, retry } = await attemptOnce(config, id, signal);
    if (!retry) {
      return outcome;
    }
    last = outcome;
  }
  return last;
}
