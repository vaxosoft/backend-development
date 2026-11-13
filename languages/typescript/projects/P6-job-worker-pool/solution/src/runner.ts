import { setTimeout as sleep } from 'node:timers/promises';
import type { JobRequest } from './schema.ts';

/**
 * Runs one attempt of a job and resolves with its result text.
 * Must reject when `signal` aborts (cancellation or timeout).
 */
export type JobRunner = (request: JobRequest, signal: AbortSignal) => Promise<string>;

export const runJob: JobRunner = async (request, signal) => {
  switch (request.type) {
    case 'reverse':
      // Spreading iterates code points; split('') would cut emoji in half.
      return [...request.payload.text].reverse().join('');
    case 'sleep':
      await sleep(request.payload.ms, undefined, { signal });
      return `slept ${request.payload.ms}ms`;
    case 'fail':
      throw new Error('job failed on purpose');
  }
};
