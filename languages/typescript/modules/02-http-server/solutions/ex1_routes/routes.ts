import Fastify, { type FastifyInstance } from 'fastify';

/**
 * Builds the app (without listening). Routes:
 * - GET /healthz           -> 200 {"status":"ok"}
 * - GET /hello             -> 200 text/plain "Hello, world!"
 * - GET /hello/:name       -> 200 text/plain "Hello, <name>!"; with ?shout=true the whole text is upper-cased
 * - GET /items/:id         -> 200 {"id": <number>}; a non-integer id -> 400 {"error": "id must be an integer"}
 * - POST /echo             -> 201 {"received": <parsed JSON body>}
 * - any other route        -> 404 {"error": "not found"}
 */
export function buildApp(): FastifyInstance {
  const app = Fastify();

  app.get('/healthz', async () => ({ status: 'ok' }));

  const hello = (name: string, shout: boolean): string => {
    const text = `Hello, ${name}!`;
    return shout ? text.toUpperCase() : text;
  };

  app.get<{ Querystring: { shout?: string } }>('/hello', async (req, reply) => {
    return reply.type('text/plain').send(hello('world', req.query.shout === 'true'));
  });

  app.get<{ Params: { name: string }; Querystring: { shout?: string } }>(
    '/hello/:name',
    async (req, reply) => {
      return reply.type('text/plain').send(hello(req.params.name, req.query.shout === 'true'));
    },
  );

  app.get<{ Params: { id: string } }>('/items/:id', async (req, reply) => {
    // Number('') is 0 and Number(' 7 ') is 7, so check the text before converting.
    if (!/^-?\d+$/.test(req.params.id)) {
      return reply.code(400).send({ error: 'id must be an integer' });
    }
    return { id: Number(req.params.id) };
  });

  app.post('/echo', async (req, reply) => reply.code(201).send({ received: req.body }));

  app.setNotFoundHandler(async (_req, reply) => reply.code(404).send({ error: 'not found' }));

  return app;
}
