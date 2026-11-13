// Hooks, plugins and encapsulation, decorators, CORS, cookies and content negotiation.

import { randomUUID } from 'node:crypto';
import cookie from '@fastify/cookie';
import cors from '@fastify/cors';
import Fastify, { type FastifyPluginAsync } from 'fastify';
import fp from 'fastify-plugin';
import { closeOnSignal, listenAddress } from './_shared.ts';

declare module 'fastify' {
  interface FastifyRequest {
    requestId: string;
  }
}

// `fp` removes encapsulation: the hook and decorator become visible to the whole app.
const requestId = fp(async (app) => {
  app.decorateRequest('requestId', '');
  app.addHook('onRequest', async (req) => {
    const incoming = req.headers['x-request-id'];
    req.requestId = typeof incoming === 'string' && incoming !== '' ? incoming : randomUUID();
  });
  app.addHook('onSend', async (req, reply) => {
    reply.header('x-request-id', req.requestId);
  });
});

// Without `fp` this plugin is encapsulated: its hook only runs for routes registered inside it.
const adminRoutes: FastifyPluginAsync = async (app) => {
  app.addHook('onRequest', async (req, reply) => {
    if (req.headers['x-admin'] !== 'yes') {
      return reply.code(403).send({ error: 'admins only' }); // returning the reply stops the chain
    }
  });
  app.get('/stats', async (req) => ({ users: 3, requestId: req.requestId }));
};

const app = Fastify();

// Hooks run in registration order, so the request-id hook goes first (CORS answers preflights early).
await app.register(requestId);
await app.register(cors, { origin: ['https://app.example.com'] });
await app.register(cookie);
await app.register(adminRoutes, { prefix: '/admin' });

app.get('/ping', async (req) => ({ pong: true, requestId: req.requestId }));

// Cookies: HttpOnly keeps the value away from page scripts; SameSite limits cross-site sending.
app.get('/visit', async (req, reply) => {
  const visits = Number(req.cookies.visits ?? '0') + 1;
  reply.setCookie('visits', String(visits), { path: '/', httpOnly: true, sameSite: 'lax' });
  return { visits };
});

// Content negotiation: pick the representation from the Accept header.
const tasks = [
  { id: 1, title: 'Write docs', status: 'todo' },
  { id: 2, title: 'Ship it', status: 'done' },
];
app.get('/tasks', async (req, reply) => {
  const accept = req.headers.accept ?? '*/*';
  reply.header('vary', 'accept');
  if (accept.includes('text/csv')) {
    const rows = tasks.map((task) => `${task.id},${task.title},${task.status}`);
    return reply.type('text/csv; charset=utf-8').send(['id,title,status', ...rows].join('\n'));
  }
  if (accept.includes('application/json') || accept.includes('*/*')) {
    return reply.send(tasks);
  }
  return reply.code(406).send({ error: 'supported: application/json, text/csv' });
});

const { host, port } = listenAddress();
await app.listen({ host, port });
console.log('listening');

closeOnSignal(() => app.close());
