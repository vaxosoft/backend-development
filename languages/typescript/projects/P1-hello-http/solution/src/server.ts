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
  app: FastifyInstance,
  options: RunOptions,
  signal: AbortSignal,
): Promise<RunningServer> {
  let shuttingDown = false;
  // Without this, a keep-alive connection that served the last in-flight request
  // would stay open until its idle timeout and delay shutdown.
  app.addHook('onSend', async (_req, reply) => {
    if (shuttingDown) reply.header('connection', 'close');
  });

  await app.listen({ host: options.host, port: options.port });
  const address = app.server.address();
  const port = typeof address === 'object' && address ? address.port : options.port;

  const aborted = signal.aborted
    ? Promise.resolve()
    : new Promise<void>((resolve) =>
        signal.addEventListener('abort', () => resolve(), { once: true }),
      );

  const stopped = aborted.then(async () => {
    shuttingDown = true;
    // app.close() stops listening at once and waits for in-flight requests.
    const force = setTimeout(() => app.server.closeAllConnections(), options.graceMs);
    try {
      await app.close();
    } finally {
      clearTimeout(force);
    }
  });

  return { port, stopped };
}
