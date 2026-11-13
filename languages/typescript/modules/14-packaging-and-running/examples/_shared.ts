// Shared by the server examples: ADDR parsing and "run until SIGINT/SIGTERM, then drain".
import { setTimeout as sleep } from 'node:timers/promises';
import type { FastifyInstance } from 'fastify';

export function parseAddr(addr: string): { host: string; port: number } {
  const index = addr.lastIndexOf(':');
  const port = Number(addr.slice(index + 1));
  if (index <= 0 || !Number.isInteger(port) || port < 0 || port > 65535) {
    throw new Error(`invalid ADDR "${addr}", expected host:port`);
  }
  return { host: addr.slice(0, index), port };
}

export interface RunOptions {
  addr: string;
  /** Hard limit for the whole shutdown (including `drainDelayMs`); after this the process exits with code 1. */
  shutdownTimeoutMs?: number;
  /** Called as soon as a signal arrives, before connections are closed. */
  onDraining?: () => void;
  /** Keep serving this long after `onDraining` so a load balancer can notice the failing readiness check. */
  drainDelayMs?: number;
}

export async function runService(app: FastifyInstance, options: RunOptions): Promise<void> {
  const { host, port } = parseAddr(options.addr);
  let draining = false;
  // Tell keep-alive clients to hang up, otherwise their idle sockets delay the shutdown.
  app.addHook('onSend', async (_request, reply) => {
    if (draining) reply.header('connection', 'close');
  });
  await app.listen({ host, port });
  console.log('listening');

  const signal = await new Promise<NodeJS.Signals>((resolve) => {
    process.once('SIGINT', resolve);
    process.once('SIGTERM', resolve);
  });
  console.log(`received ${signal}, draining`);
  draining = true;
  options.onDraining?.();

  const forceExit = setTimeout(() => {
    console.error('drain timeout, forcing exit');
    process.exit(1);
  }, options.shutdownTimeoutMs ?? 10_000);
  forceExit.unref();

  if (options.drainDelayMs) await sleep(options.drainDelayMs);
  await app.close();
  console.log('stopped');
}
