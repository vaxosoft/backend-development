// CPU-bound work blocks the event loop; a worker thread moves it off the main thread.
import { setTimeout as sleep } from 'node:timers/promises';
import { isMainThread, parentPort, Worker, workerData } from 'node:worker_threads';

function fib(n: number): number {
  return n < 2 ? n : fib(n - 1) + fib(n - 2);
}

if (isMainThread) {
  // A heartbeat that should tick every millisecond shows whether the loop is responsive.
  async function measureTicks(
    work: () => Promise<number>,
  ): Promise<{ value: number; ticks: number }> {
    let ticks = 0;
    const timer = setInterval(() => ticks++, 1);
    const value = await work();
    clearInterval(timer);
    return { value, ticks };
  }

  const inline = await measureTicks(async () => {
    await sleep(1); // let the interval start
    return fib(36); // runs on the main thread: nothing else can run meanwhile
  });
  console.log(
    'main thread:',
    inline.value,
    'heartbeat ticks:',
    inline.ticks > 5 ? 'many' : 'almost none',
  );

  const offloaded = await measureTicks(
    () =>
      new Promise<number>((resolve, reject) => {
        const worker = new Worker(import.meta.filename, { workerData: 36 });
        worker.once('message', resolve);
        worker.once('error', reject); // an exception inside the worker arrives here
      }),
  );
  console.log(
    'worker thread:',
    offloaded.value,
    'heartbeat ticks:',
    offloaded.ticks > 5 ? 'many' : 'almost none',
  );
} else {
  // Worker side: the same file, different branch. Data is copied (structured clone) in both directions.
  parentPort?.postMessage(fib(workerData as number));
}
