import type { FastifyInstance } from 'fastify';
import type { Logger } from 'pino';

export function registerRequestLogging(app: FastifyInstance, logger: Logger): void {
  app.addHook('onResponse', async (request, reply) => {
    logger.info(
      {
        request_id: request.id,
        method: request.method,
        path: request.url.split('?')[0],
        status: reply.statusCode,
        duration_ms: Math.round(reply.elapsedTime * 100) / 100,
        user_id: request.userId ?? undefined,
      },
      'request',
    );
  });
}
