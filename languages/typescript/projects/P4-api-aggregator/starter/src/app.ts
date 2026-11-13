import Fastify, { type FastifyInstance, type FastifyReply } from 'fastify';
import { aggregate } from './aggregator.ts';
import type { Config } from './config.ts';
import { parseIds } from './ids.ts';

function sendError(reply: FastifyReply, status: number, code: string, message: string) {
  return reply.code(status).send({ error: { code, message } });
}

export async function buildApp(
  config: Config,
  options: { logger?: boolean } = {},
): Promise<FastifyInstance> {
  const app = Fastify({ logger: options.logger ?? false });

  app.setNotFoundHandler((_request, reply) => sendError(reply, 404, 'not_found', 'not found'));
  app.setErrorHandler((error, request, reply) => {
    request.log.error({ err: error }, 'unhandled error');
    return sendError(reply, 500, 'internal', 'internal server error');
  });

  app.get<{ Querystring: { ids?: unknown } }>('/aggregate', async (request, reply) => {
    const parsed = parseIds(request.query.ids);
    if (!parsed.ok) {
      return sendError(reply, 400, 'bad_request', parsed.message);
    }

    // Aborted when the client goes away before the response was fully written.
    const clientGone = new AbortController();
    reply.raw.once('close', () => {
      if (!reply.raw.writableFinished) {
        clientGone.abort();
      }
    });

    return aggregate(parsed.ids, config, clientGone.signal);
  });

  return app;
}
