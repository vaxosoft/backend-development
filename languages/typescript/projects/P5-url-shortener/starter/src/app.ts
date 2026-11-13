import { randomUUID } from 'node:crypto';
import Fastify, { type FastifyInstance } from 'fastify';
import pino from 'pino';
import type { Config } from './config.ts';
import { openDatabase } from './db.ts';
import { registerErrorHandling } from './errors.ts';
import { TokenBucketLimiter } from './rate-limit.ts';
import { registerRequestLogging } from './request-log.ts';
import { registerRoutes } from './routes.ts';
import { Store } from './store.ts';

export async function buildApp(config: Config): Promise<FastifyInstance> {
  const db = openDatabase(config.dbPath);
  const store = new Store(db);
  const limiter = new TokenBucketLimiter(
    config.rateLimitBurst,
    config.rateLimitRefillPerSecond,
    config.now,
  );
  // Belt and braces: nothing logs these keys, and pino would censor them if something did.
  const logger = pino(
    { base: null, redact: { paths: ['password', 'token', 'authorization'], censor: '[redacted]' } },
    config.logStream,
  );

  const app = Fastify({
    requestIdHeader: 'x-request-id',
    genReqId: () => randomUUID(),
  });
  app.decorateRequest('userId', null);
  app.addHook('onSend', async (request, reply) => {
    reply.header('x-request-id', request.id);
  });
  app.addHook('onClose', async () => {
    db.close();
  });

  registerErrorHandling(app, logger);
  registerRequestLogging(app, logger);
  await registerRoutes(app, { config, store, limiter });
  return app;
}
