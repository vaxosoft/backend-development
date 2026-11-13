import { AsyncLocalStorage } from 'node:async_hooks';
import { setTimeout as sleep } from 'node:timers/promises';
import pino from 'pino';

interface RequestContext {
  requestId: string;
}

const storage = new AsyncLocalStorage<RequestContext>();

// The mixin runs on every log call and merges its result into the line, so code deep in the
// call stack never has to receive a logger or a request id as a parameter.
const logger = pino(
  { timestamp: false, base: null, mixin: () => ({ request_id: storage.getStore()?.requestId }) },
  process.stdout,
);

async function loadTasks(): Promise<void> {
  logger.info('loading tasks');
  await sleep(5);
  logger.info('tasks loaded');
}

async function handle(requestId: string, delayMs: number): Promise<void> {
  await storage.run({ requestId }, async () => {
    logger.info('request started');
    await sleep(delayMs);
    await loadTasks();
    logger.info('request finished');
  });
}

logger.info('outside any request');
await Promise.all([handle('req-a', 10), handle('req-b', 80)]);
