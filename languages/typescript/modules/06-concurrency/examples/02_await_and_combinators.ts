// Concurrency in Node is overlapping waits: await in a loop serialises them, Promise combinators overlap them.
import { setTimeout as sleep } from 'node:timers/promises';

async function fetchTitle(id: number, ms: number): Promise<string> {
  await sleep(ms);
  if (id === 3) throw new Error(`task ${id} failed`);
  return `task ${id}`;
}

async function timed<T>(label: string, work: () => Promise<T>): Promise<T> {
  const start = performance.now();
  const value = await work();
  const elapsed = performance.now() - start;
  // Coarse buckets keep the output deterministic.
  console.log(`${label}: ${elapsed >= 150 ? 'slow (~sum of the waits)' : 'fast (~longest wait)'}`);
  return value;
}

const ids = [1, 2, 4, 5];

// Anti-pattern: each iteration waits for the previous one.
const sequential = await timed('await in a loop', async () => {
  const out: string[] = [];
  for (const id of ids) out.push(await fetchTitle(id, 50));
  return out;
});

// Start everything first, then wait. Results keep the input order.
const parallel = await timed('Promise.all', () => Promise.all(ids.map((id) => fetchTitle(id, 50))));
console.log(sequential.join() === parallel.join(), parallel);

// Promise.all is fail-fast: the first rejection wins, the other promises keep running unobserved.
try {
  await Promise.all([1, 3, 2].map((id) => fetchTitle(id, id * 10)));
} catch (err: unknown) {
  if (err instanceof Error) console.log('Promise.all rejected:', err.message);
}

// allSettled waits for all of them and reports each outcome.
const settled = await Promise.allSettled([1, 3, 2].map((id) => fetchTitle(id, 10)));
console.log(
  settled.map((r) => (r.status === 'fulfilled' ? r.value : `rejected: ${r.reason.message}`)),
);

// race: first to settle (success OR failure). any: first to succeed.
console.log('race:', await Promise.race([fetchTitle(1, 30), fetchTitle(2, 10)]));
console.log('any:', await Promise.any([fetchTitle(3, 5), fetchTitle(2, 20), fetchTitle(1, 40)]));

// A promise that is started but never awaited: the caller moves on before the work finishes, and a
// rejection would surface as an unhandled rejection that terminates the process.
let finished = false;
const forgotten = (async () => {
  await sleep(10);
  finished = true;
})();
console.log('finished right after starting:', finished);
await forgotten;
console.log('finished after awaiting:', finished);

// One thread is not the same as no races: a read-modify-write that spans an await interleaves.
let counter = 0;
async function unsafeIncrement(): Promise<void> {
  const current = counter;
  await sleep(1);
  counter = current + 1; // overwrites the increments made by the other calls
}
await Promise.all([unsafeIncrement(), unsafeIncrement(), unsafeIncrement()]);
console.log('counter after 3 unsafe increments:', counter);

counter = 0;
const safeIncrement = async (): Promise<void> => {
  await sleep(1);
  counter++; // read and write in the same synchronous step: nothing can interleave
};
await Promise.all([safeIncrement(), safeIncrement(), safeIncrement()]);
console.log('counter after 3 safe increments:', counter);
