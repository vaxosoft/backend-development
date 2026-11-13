import type { FastifyInstance } from 'fastify';
import type { ListenAddress } from './config.ts';

export interface RunOptions extends ListenAddress {
  /** How long in-flight requests may keep running after shutdown starts. */
  graceMs: number;
}

export interface RunningServer {
  /** The port actually bound (useful when `port` was 0). */
  port: number;
  /** Resolves once the server has fully shut down. */
  stopped: Promise<void>;
}

/**
 * Listen, then shut down gracefully when `signal` aborts: stop accepting
 * connections, let in-flight requests finish, force-close after `graceMs`.
 */
export async function run(
  _app: FastifyInstance,
  _options: RunOptions,
  _signal: AbortSignal,
): Promise<RunningServer> {
  throw new Error('TODO');
}
