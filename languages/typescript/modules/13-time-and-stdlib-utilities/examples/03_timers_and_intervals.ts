// Timers and in-process scheduling with node:timers/promises.
import { setInterval as every, setTimeout as sleep } from 'node:timers/promises';

// 1. Promise-based sleep that can resolve with a value.
const started = performance.now();
const value = await sleep(20, 'woke up');
console.log(value, '- waited at least 15 ms:', performance.now() - started >= 15);

// 2. A timer is cancelled with an AbortSignal; the promise rejects with an AbortError.
const controller = new AbortController();
const longSleep = sleep(60_000, undefined, { signal: controller.signal });
controller.abort();
try {
  await longSleep;
} catch (error) {
  console.log('cancelled sleep ->', error instanceof Error ? error.name : 'unknown');
}

// 3. Intervals are async iterators: `break` stops the timer, no clearInterval needed.
let ticks = 0;
for await (const _ of every(5)) {
  ticks += 1;
  if (ticks === 3) break;
}
console.log('interval ticks:', ticks);

// 4. setInterval starts a new run even if the previous one is still busy; a loop does not.
async function maxConcurrency(mode: 'setInterval' | 'loop'): Promise<number> {
  let running = 0;
  let max = 0;
  const job = async () => {
    running += 1;
    max = Math.max(max, running);
    await sleep(30);
    running -= 1;
  };
  const stop = new AbortController();
  setTimeout(() => stop.abort(), 80);
  if (mode === 'setInterval') {
    const timer = setInterval(() => void job(), 5);
    await sleep(80);
    clearInterval(timer);
    await sleep(40); // let in-flight runs finish
  } else {
    while (!stop.signal.aborted) {
      await job();
      await sleep(5);
    }
  }
  return max;
}
console.log('setInterval overlaps:', (await maxConcurrency('setInterval')) > 1);
console.log('sequential loop overlaps:', (await maxConcurrency('loop')) > 1);

// 5. A background timer must not keep the process alive on shutdown: unref() it.
const housekeeping = setInterval(() => console.log('never printed'), 60_000);
housekeeping.unref();
console.log('exiting without waiting for the unref()ed timer');
