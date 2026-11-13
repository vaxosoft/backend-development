import { z } from 'zod';
import { createApp, errorEnvelope, Metrics, registerHealth } from './_shared.ts';

const NewTask = z.object({ title: z.string().trim().min(1).max(200) });

interface Task {
  id: number;
  title: string;
  status: 'todo';
}

const metrics = new Metrics();
const app = createApp({ sink: process.stdout, onCompleted: (r) => metrics.record(r) });
registerHealth(app, metrics);

const tasks = new Map<number, Task>();

app.post('/tasks', async (req, reply) => {
  const parsed = NewTask.safeParse(req.body);
  if (!parsed.success) {
    return reply.code(422).send({
      error: {
        ...errorEnvelope('validation_failed', 'invalid task').error,
        fields: { title: 'must be 1-200 characters' },
      },
    });
  }
  const task: Task = { id: tasks.size + 1, title: parsed.data.title, status: 'todo' };
  tasks.set(task.id, task);
  req.log.info({ task_id: task.id }, 'task created');
  return reply.code(201).header('location', `/tasks/${task.id}`).send(task);
});

app.get('/tasks/:id', async (req, reply) => {
  const task = tasks.get(Number((req.params as { id: string }).id));
  return task ?? reply.code(404).send(errorEnvelope('not_found', 'task not found'));
});

app.get('/panic', async () => {
  throw new Error('simulated bug');
});

// After an unhandled rejection or uncaught exception the process state is unknown:
// record it, then stop and let the supervisor restart us.
function crash(kind: string) {
  return (err: unknown) => {
    app.log.fatal({ err }, kind);
    process.exit(1);
  };
}
process.on('uncaughtException', crash('uncaught exception'));
process.on('unhandledRejection', crash('unhandled rejection'));

async function shutdown(signal: string): Promise<void> {
  app.log.info({ signal }, 'shutting down');
  await app.close();
}
process.once('SIGINT', () => void shutdown('SIGINT'));
process.once('SIGTERM', () => void shutdown('SIGTERM'));

const [host = '127.0.0.1', port = '8080'] = (process.env.ADDR ?? '127.0.0.1:8080').split(':');
await app.listen({ host, port: Number(port) });
