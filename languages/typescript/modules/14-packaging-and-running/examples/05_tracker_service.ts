// The Task Tracker service as it would ship: env-driven settings, build info, liveness vs readiness.
// Run:  ADDR=127.0.0.1:22402 APP_VERSION=1.4.0 GIT_COMMIT=3f9c2ab node modules/14-packaging-and-running/examples/05_tracker_service.ts
import Fastify from 'fastify';
import { runService } from './_shared.ts';

const buildInfo = {
  name: 'task-tracker',
  version: process.env.APP_VERSION ?? 'dev',
  commit: process.env.GIT_COMMIT ?? 'unknown',
};

const tasks = [
  {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    due_date: '2030-01-31',
    tags: ['docs'],
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z',
  },
  {
    id: 2,
    title: 'Ship release',
    status: 'in_progress',
    due_date: null,
    tags: [],
    owner_id: 1,
    created_at: '2030-01-02T10:30:00Z',
  },
];

const app = Fastify({ requestTimeout: 15_000, keepAliveTimeout: 5_000 });
let ready = true;

// Liveness: "the process is up". Orchestrators restart the container when this fails.
app.get('/healthz', async () => ({ status: 'ok' }));

// Readiness: "send me traffic". It fails while draining so the load balancer stops routing here.
app.get('/readyz', async (_request, reply) => {
  if (!ready) return reply.code(503).send({ status: 'draining' });
  return { status: 'ready' };
});

app.get('/version', async () => buildInfo);

app.get('/tasks', async () => ({ items: tasks, total: tasks.length, limit: 20, offset: 0 }));

app.setNotFoundHandler((_request, reply) =>
  reply.code(404).send({ error: { code: 'not_found', message: 'route not found' } }),
);

await runService(app, {
  addr: process.env.ADDR ?? '127.0.0.1:8080',
  drainDelayMs: Number(process.env.DRAIN_DELAY_MS ?? 0),
  onDraining: () => {
    ready = false;
  },
});
