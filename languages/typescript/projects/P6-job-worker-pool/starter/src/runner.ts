import type { JobRequest } from './schema.ts';

/**
 * Runs one attempt of a job and resolves with its result text.
 * Must reject when `signal` aborts (cancellation or timeout).
 */
export type JobRunner = (request: JobRequest, signal: AbortSignal) => Promise<string>;

/**
 * TODO:
 * - reverse: the text reversed by code points (emoji must survive)
 * - sleep: wait `ms` but stop as soon as `signal` aborts; result `slept <ms>ms`
 * - fail: always reject with Error('job failed on purpose')
 */
export const runJob: JobRunner = async (_request, _signal) => {
  throw new Error('TODO');
};
