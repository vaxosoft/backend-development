// Limiting concurrency by hand and cancelling work with AbortSignal.
import { setTimeout as sleep } from 'node:timers/promises';

/** Runs `fn` over `items` with at most `limit` calls in flight; results keep the input order. */
async function mapLimit<T, R>(
  items: readonly T[],
  limit: number,
  fn: (item: T, index: number) => Promise<R>,
): Promise<R[]> {
  const results = new Array<R>(items.length);
  let next = 0;
  // `limit` runners share one cursor; JavaScript is single threaded, so `next++` cannot race.
  const runner = async (): Promise<void> => {
    while (next < items.length) {
      const index = next++;
      results[index] = await fn(items[index] as T, index);
    }
  };
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, runner));
  return results;
}

let inFlight = 0;
let peak = 0;
const doubled = await mapLimit([1, 2, 3, 4, 5, 6, 7, 8], 3, async (n) => {
  inFlight++;
  peak = Math.max(peak, inFlight);
  await sleep(10);
  inFlight--;
  return n * 2;
});
console.log(doubled.join(), 'peak in flight:', peak);

// Cancellation: a signal is just an event source that long-running code must listen to.
async function step(name: string, signal: AbortSignal): Promise<string> {
  await sleep(30, undefined, { signal }); // rejects as soon as the signal aborts
  return name;
}

const controller = new AbortController();
const started = step('first', controller.signal);
setTimeout(() => controller.abort(), 5);
try {
  await started;
} catch (err: unknown) {
  if (err instanceof Error) console.log('cancelled:', err.name);
}

// AbortSignal.timeout() aborts by itself. Node's timers wrap the signal's reason: the error is an
// AbortError whose `cause` is the TimeoutError (fetch rejects with the TimeoutError itself).
try {
  await step('slow', AbortSignal.timeout(5));
} catch (err: unknown) {
  if (err instanceof Error) console.log('timed out:', err.name, '<-', (err.cause as Error).name);
}

// AbortSignal.any() combines "the caller gave up" with "we ran out of time".
const caller = new AbortController();
const combined = AbortSignal.any([caller.signal, AbortSignal.timeout(1000)]);
caller.abort(new Error('user left'));
console.log('combined aborted:', combined.aborted, '-', (combined.reason as Error).message);

// A loop with no natural await point on the signal must check it between steps.
const stopper = new AbortController();
async function countUntilAborted(signal: AbortSignal): Promise<number> {
  let n = 0;
  while (true) {
    signal.throwIfAborted();
    n++;
    await sleep(1);
    if (n === 3) stopper.abort();
  }
}
try {
  await countUntilAborted(stopper.signal);
} catch (err: unknown) {
  if (err instanceof Error) console.log('loop stopped:', err.name);
}

// mapLimit has no cancellation of its own: on the first rejection Promise.all rejects, but the
// other runners keep going until the iterator is exhausted.
const startedItems: number[] = [];
try {
  await mapLimit([1, 2, 3, 4, 5, 6], 2, async (n) => {
    startedItems.push(n);
    await sleep(5);
    if (n === 1) throw new Error('boom');
    return n;
  });
} catch (err: unknown) {
  if (err instanceof Error) console.log('rejected:', err.message);
}
await sleep(40);
console.log('items started anyway:', startedItems);
