import type { FastifyBaseLogger } from 'fastify';
import pino from 'pino';

export interface Config {
  workers: number;
  /** Jobs that may wait for a free worker. Running jobs do not count. */
  queueCapacity: number;
  maxAttempts: number;
  retryDelayMs: number;
  /** Limit for one attempt of one job. */
  jobTimeoutMs: number;
  /** How long shutdown waits for running and queued jobs before cancelling them. */
  drainTimeoutMs: number;
  logger: FastifyBaseLogger;
}

export type PoolConfig = Omit<Config, 'logger'>;

export const defaultConfig: Config = {
  workers: 4,
  queueCapacity: 100,
  maxAttempts: 3,
  retryDelayMs: 200,
  jobTimeoutMs: 10_000,
  drainTimeoutMs: 10_000,
  logger: pino({ level: 'silent' }),
};

export function parseAddr(addr: string): { host: string; port: number } {
  const at = addr.lastIndexOf(':');
  return { host: addr.slice(0, at), port: Number(addr.slice(at + 1)) };
}
