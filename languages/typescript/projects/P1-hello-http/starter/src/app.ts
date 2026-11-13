import Fastify, { type FastifyInstance } from 'fastify';
import { type Config, withDefaults } from './config.ts';
import { registerRecovery, registerRequestId, registerRequestLogger } from './middleware.ts';
import { registerRoutes } from './routes.ts';

export async function buildApp(partial: Partial<Config> = {}): Promise<FastifyInstance> {
  const config = withDefaults(partial);

  const app = Fastify({
    // TODO: configure the server timeouts from `config` (requestTimeout,
    // keepAliveTimeout and http.headersTimeout) and let Fastify use the
    // incoming X-Request-ID header (requestIdHeader + genReqId).
  });

  registerRequestId(app);
  registerRequestLogger(app, config.logSink);
  registerRecovery(app, config.errorSink);
  registerRoutes(app);
  return app;
}
