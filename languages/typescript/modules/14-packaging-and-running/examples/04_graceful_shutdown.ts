// Production process lifecycle: server timeouts, SIGTERM handling and a graceful drain.
// Run:  ADDR=127.0.0.1:22400 node modules/14-packaging-and-running/examples/04_graceful_shutdown.ts
import { setTimeout as sleep } from 'node:timers/promises';
import Fastify from 'fastify';
import { runService } from './_shared.ts';

const app = Fastify({
  connectionTimeout: 30_000, // socket idle timeout
  keepAliveTimeout: 5_000, // keep-alive sockets are closed after this long without a request
  requestTimeout: 15_000, // whole request must arrive within this time
  forceCloseConnections: 'idle', // on close() drop idle keep-alive sockets, let busy ones finish
});

app.get('/healthz', async () => ({ status: 'ok' }));

app.get<{ Querystring: { ms?: string } }>('/slow', async (request) => {
  const ms = Math.min(Number(request.query.ms ?? 0) || 0, 2000);
  await sleep(ms);
  return { waited_ms: ms };
});

process.on('unhandledRejection', (reason) => {
  console.error('unhandled rejection, exiting:', reason);
  process.exit(1);
});

await runService(app, {
  addr: process.env.ADDR ?? '127.0.0.1:8080',
  shutdownTimeoutMs: 5_000,
});
