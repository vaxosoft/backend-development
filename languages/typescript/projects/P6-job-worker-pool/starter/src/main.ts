import pino from 'pino';
import { buildApp } from './app.ts';
import { type Config, parseAddr } from './config.ts';

function intFromEnv(name: string): number | undefined {
  const raw = process.env[name];
  return raw === undefined ? undefined : Number(raw);
}

const overrides: Partial<Config> = { logger: pino({ level: process.env.LOG_LEVEL ?? 'info' }) };
for (const [key, env] of [
  ['workers', 'WORKERS'],
  ['queueCapacity', 'QUEUE_CAPACITY'],
  ['maxAttempts', 'MAX_ATTEMPTS'],
  ['retryDelayMs', 'RETRY_DELAY_MS'],
  ['jobTimeoutMs', 'JOB_TIMEOUT_MS'],
  ['drainTimeoutMs', 'DRAIN_TIMEOUT_MS'],
] as const) {
  const value = intFromEnv(env);
  if (value !== undefined) {
    overrides[key] = value;
  }
}

const { host, port } = parseAddr(process.env.ADDR ?? '127.0.0.1:8080');
const app = await buildApp(overrides);

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.once(signal, () => {
    app.log.info({ signal }, 'shutting down');
    app.close().then(
      () => process.exit(0),
      () => process.exit(1),
    );
  });
}

await app.listen({ host, port });
