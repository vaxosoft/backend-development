import Fastify, { type FastifyInstance, type FastifyPluginAsync } from 'fastify';

declare module 'fastify' {
  interface FastifyRequest {
    requestId: string;
  }
}

/**
 * Request-ID plugin. It must be visible to the whole app (not encapsulated):
 * - decorate the request with `requestId` (use an empty string as the initial value)
 * - an `onRequest` hook sets `req.requestId` to the incoming `X-Request-ID` header when it matches
 *   /^[A-Za-z0-9._-]{1,64}$/, otherwise to a freshly generated UUID (`crypto.randomUUID()`)
 * - an `onSend` hook echoes it in the `X-Request-ID` response header (also on errors and 404s)
 */
export const requestIdPlugin: FastifyPluginAsync = async () => {
  throw new Error('TODO');
};

/**
 * Admin plugin, registered under the prefix `/admin`. It must stay encapsulated:
 * an `onRequest` hook answers 401 `{"error":"unauthorized"}` unless the header `x-api-key` is `secret`,
 * and `GET /ping` (that is `/admin/ping`) answers `{"ok":true}`.
 * The hook must NOT affect routes outside the plugin.
 */
export const adminPlugin: FastifyPluginAsync = async () => {
  throw new Error('TODO');
};

export function buildApp(): FastifyInstance {
  const app = Fastify();
  app.register(requestIdPlugin);
  app.register(adminPlugin, { prefix: '/admin' });
  app.get('/public', async () => ({ visible: true }));
  app.get('/whoami', async (req) => ({ requestId: req.requestId }));
  return app;
}
