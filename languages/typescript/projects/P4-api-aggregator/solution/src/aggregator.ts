import type { Config } from './config.ts';
import {
  type AggregateResponse,
  DEADLINE_EXCEEDED,
  type FetchOutcome,
  type ItemError,
  type ItemResult,
} from './types.ts';
import { fetchItem } from './upstream.ts';

export async function aggregate(
  ids: number[],
  config: Config,
  clientSignal: AbortSignal,
): Promise<AggregateResponse> {
  const signal = AbortSignal.any([clientSignal, AbortSignal.timeout(config.overallTimeoutMs)]);
  const outcomes: FetchOutcome[] = ids.map(() => DEADLINE_EXCEEDED);

  // The workers share one iterator, so each id is handed to exactly one of them.
  const queue = ids.entries();
  const worker = async () => {
    for (const [index, id] of queue) {
      if (signal.aborted) {
        return;
      }
      outcomes[index] = await fetchItem(config, id, signal);
    }
  };
  const workerCount = Math.min(config.maxConcurrency, ids.length);
  await Promise.all(Array.from({ length: workerCount }, worker));

  const results: ItemResult[] = [];
  const errors: ItemError[] = [];
  ids.forEach((id, index) => {
    const outcome = outcomes[index] ?? DEADLINE_EXCEEDED;
    if (outcome.ok) {
      results.push({ id, data: outcome.data });
    } else {
      errors.push({ id, error: outcome.error });
    }
  });
  return { results, errors };
}
