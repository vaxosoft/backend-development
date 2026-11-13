// A small Task Tracker server: routing, Location header, server timeouts and graceful shutdown.

import { setTimeout as sleep } from 'node:timers/promises';
import Fastify from 'fastify';
import { closeOnSignal, listenAddress } from './_shared.ts';

interface Task {
  id: number;
  title: string;
  status: 'todo' | 'in_progress' | 'done';
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

const tasks = new Map<number, Task>();
let nextId = 1;

function addTask(title: string): Task {
  const task: Task = {
    id: nextId++,
    title,
    status: 'todo',
    due_date: null,
    tags: [],
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z',
  };
  tasks.set(task.id, task);
  return task;
}
addTask('Write docs');

const app = Fastify({
  connectionTimeout: 30_000, // idle sockets without a request are destroyed
  keepAliveTimeout: 5_000,
  requestTimeout: 30_000, // the client must send the whole request within 30 s
  forceCloseConnections: 'idle', // on close(), drop idle keep-alive sockets but let busy ones finish
});

app.get('/healthz', async () => ({ status: 'ok' }));

app.get('/tasks', async () => [...tasks.values()]);

app.get<{ Params: { id: string } }>('/tasks/:id', async (req, reply) => {
  const task = tasks.get(Number(req.params.id));
  if (!task) return reply.code(404).send({ error: 'task not found' });
  return task;
});

app.post<{ Body: { title?: unknown } }>('/tasks', async (req, reply) => {
  const title = typeof req.body?.title === 'string' ? req.body.title.trim() : '';
  if (title === '') return reply.code(400).send({ error: 'title is required' });
  const task = addTask(title);
  return reply.code(201).header('location', `/tasks/${task.id}`).send(task);
});

app.delete<{ Params: { id: string } }>('/tasks/:id', async (req, reply) => {
  if (!tasks.delete(Number(req.params.id)))
    return reply.code(404).send({ error: 'task not found' });
  return reply.code(204).send();
});

// Used to try graceful shutdown: send SIGTERM while this request is still running.
app.get<{ Querystring: { ms?: string } }>('/slow', async (req) => {
  await sleep(Math.min(Number(req.query.ms ?? '1000') || 1000, 5000));
  return { done: true };
});

app.addHook('onClose', async () => {
  console.log('closed: no more connections');
});

const { host, port } = listenAddress();
await app.listen({ host, port });
console.log('listening');

closeOnSignal(() => app.close());
