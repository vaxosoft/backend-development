import type { FastifyError, FastifyInstance } from 'fastify';
import { errorBody } from './errors.ts';

/** Echo the request's X-Request-ID on every response (generated when absent). */
export function registerRequestId(app: FastifyInstance): void {
  app.addHook('onRequest', async (req, reply) => {
    reply.header('x-request-id', req.id);
  });
}

/** Write one JSON line per finished request to `sink`. */
export function registerRequestLogger(app: FastifyInstance, sink: (line: string) => void): void {
  app.addHook('onResponse', async (req, reply) => {
    sink(
      JSON.stringify({
        request_id: req.id,
        method: req.method,
        path: req.url.split('?')[0],
        status: reply.statusCode,
        duration_ms: Math.round(reply.elapsedTime * 100) / 100,
      }),
    );
  });
}

const CLIENT_ERROR_CODES: Record<number, string> = {
  400: 'bad_request',
  404: 'not_found',
  413: 'payload_too_large',
};

/** Turn thrown errors and unknown routes into the shared JSON error envelope. */
export function registerRecovery(app: FastifyInstance, errorSink: (error: unknown) => void): void {
  app.setErrorHandler(async (error: FastifyError, _req, reply) => {
    const status = error.statusCode ?? 500;
    if (status >= 500) {
      errorSink(error);
      return reply.code(500).send(errorBody('internal', 'internal server error'));
    }
    return reply
      .code(status)
      .send(errorBody(CLIENT_ERROR_CODES[status] ?? 'bad_request', error.message));
  });

  app.setNotFoundHandler(async (req, reply) =>
    reply.code(404).send(errorBody('not_found', `no route for ${req.method} ${req.url}`)),
  );
}
