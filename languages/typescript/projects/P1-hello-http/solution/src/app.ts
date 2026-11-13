import { randomUUID } from 'node:crypto';
import Fastify, { type FastifyInstance } from 'fastify';
import { type Config, withDefaults } from './config.ts';
import { registerRecovery, registerRequestId, registerRequestLogger } from './middleware.ts';
import { registerRoutes } from './routes.ts';

export async function buildApp(partial: Partial<Config> = {}): Promise<FastifyInstance> {
  const config = withDefaults(partial);

  const app = Fastify({
    requestTimeout: config.requestTimeoutMs,
    keepAliveTimeout: config.keepAliveTimeoutMs,
    http: { headersTimeout: config.headersTimeoutMs },
    requestIdHeader: 'x-request-id',
    genReqId: () => randomUUID(),
  });

  registerRequestId(app);
  registerRequestLogger(app, config.logSink);
  registerRecovery(app, config.errorSink);
  registerRoutes(app);
  return app;
}
