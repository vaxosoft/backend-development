import type { FastifyInstance } from 'fastify';

/** An error that already knows its HTTP status and envelope code. */
export class HttpError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: Record<string, string>;

  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message);
    this.name = 'HttpError';
    this.status = status;
    this.code = code;
    if (fields) this.fields = fields;
  }
}

export const notFound = (what = 'resource') => new HttpError(404, 'not_found', `${what} not found`);

export const badRequest = (message: string) => new HttpError(400, 'bad_request', message);

export const validationFailed = (fields: Record<string, string>) =>
  new HttpError(422, 'validation_failed', 'invalid task', fields);

const CLIENT_ERROR_CODES: Record<number, string> = {
  400: 'bad_request',
  404: 'not_found',
  413: 'payload_too_large',
  415: 'unsupported_media_type',
};

function clientStatus(error: unknown): number {
  if (typeof error === 'object' && error !== null && 'statusCode' in error) {
    if (typeof error.statusCode === 'number') return error.statusCode;
  }
  return 500;
}

/** Every error leaves the API as the shared JSON envelope. */
export function registerErrorHandling(app: FastifyInstance): void {
  app.setErrorHandler(async (error, _req, reply) => {
    if (error instanceof HttpError) {
      const body = { code: error.code, message: error.message, fields: error.fields };
      return reply.code(error.status).send({ error: body });
    }
    const status = clientStatus(error);
    if (status < 500) {
      const code = CLIENT_ERROR_CODES[status] ?? 'bad_request';
      const message = error instanceof Error ? error.message : 'bad request';
      return reply.code(status).send({ error: { code, message } });
    }
    app.log.error(error);
    return reply.code(500).send({ error: { code: 'internal', message: 'internal server error' } });
  });

  app.setNotFoundHandler(async (_req, reply) =>
    reply.code(404).send({ error: { code: 'not_found', message: 'route not found' } }),
  );
}
