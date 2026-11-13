// Fastify basics: routes, path and query parameters, headers, JSON bodies, status codes.
import Fastify from 'fastify';
import { closeOnSignal, listenAddress } from './_shared.ts';

const app = Fastify();

app.get('/healthz', async () => ({ status: 'ok' })); // returned objects are sent as JSON

app.get<{ Params: { name: string } }>('/hello/:name', async (req, reply) => {
  reply.type('text/plain; charset=utf-8');
  return `Hello, ${req.params.name}!`;
});

app.get('/hello', async (_req, reply) => reply.type('text/plain').send('Hello, world!'));

// Query values are always strings (or arrays of strings) until you convert them.
app.get<{ Querystring: { page?: string } }>('/page', async (req) => {
  const page = Number(req.query.page ?? '1');
  return { page, valid: Number.isInteger(page) && page >= 1 };
});

app.get('/whoami', async (req) => ({
  userAgent: req.headers['user-agent'] ?? null,
  accept: req.headers.accept ?? null,
}));

app.post<{ Body: { message?: unknown } }>('/echo', async (req, reply) => {
  return reply.code(201).header('x-echo', 'true').send({ received: req.body });
});

app.get('/teapot', async (_req, reply) => reply.code(418).send({ message: 'short and stout' }));

const { host, port } = listenAddress();
await app.listen({ host, port });
console.log('listening');

closeOnSignal(() => app.close());
