import type { FastifyError, FastifyInstance, FastifyReply } from 'fastify';
import type { Logger } from 'pino';
import type { z } from 'zod';

export interface ErrorOptions {
  fields?: Record<string, string>;
  headers?: Record<string, string>;
}

/** An expected failure that maps to a status code and the shared error envelope. */
export class HttpError extends Error {
  readonly status: number;
  readonly code: string;
  readonly options: ErrorOptions;

  constructor(status: number, code: string, message: string, options: ErrorOptions = {}) {
    super(message);
    this.status = status;
    this.code = code;
    this.options = options;
  }
}

function send(
  reply: FastifyReply,
  status: number,
  code: string,
  message: string,
  options: ErrorOptions = {},
) {
  return reply
    .code(status)
    .headers(options.headers ?? {})
    .send({ error: { code, message, ...(options.fields && { fields: options.fields }) } });
}

export function registerErrorHandling(app: FastifyInstance, logger: Logger): void {
  app.setNotFoundHandler((_request, reply) => send(reply, 404, 'not_found', 'not found'));

  app.setErrorHandler((error: FastifyError, request, reply) => {
    if (error instanceof HttpError) {
      return send(reply, error.status, error.code, error.message, error.options);
    }
    // Fastify's own 4xx errors: malformed JSON, body too large, ...
    if (error.statusCode === 413) {
      return send(reply, 413, 'payload_too_large', 'payload too large');
    }
    if (error.statusCode !== undefined && error.statusCode >= 400 && error.statusCode < 500) {
      return send(reply, 400, 'bad_request', error.message);
    }
    logger.error({ request_id: request.id, err: error }, 'unhandled error');
    return send(reply, 500, 'internal', 'internal server error');
  });
}

/** Validate untrusted input; types are erased at runtime, so this is the real check. */
export function parseBody<T>(schema: z.ZodType<T>, body: unknown): T {
  const result = schema.safeParse(body);
  if (result.success) {
    return result.data;
  }
  const fields: Record<string, string> = {};
  for (const issue of result.error.issues) {
    fields[String(issue.path[0] ?? 'body')] ??= issue.message;
  }
  throw new HttpError(422, 'validation_failed', 'invalid request', { fields });
}
