// A small typed client for the Task Tracker API: base URL, bearer auth, timeouts, error envelope,
// and runtime validation of what the server sends back (TypeScript types are erased at runtime).
import { z } from 'zod';
import { startServer } from './_shared.ts';

const taskSchema = z.object({
  id: z.number().int(),
  title: z.string(),
  status: z.enum(['todo', 'in_progress', 'done']),
  due_date: z.string().nullable(),
  tags: z.array(z.string()),
  owner_id: z.number().int().nullable(),
  created_at: z.string(),
});
type Task = z.infer<typeof taskSchema>;

const listSchema = z.object({
  items: z.array(taskSchema),
  total: z.number().int(),
  limit: z.number().int(),
  offset: z.number().int(),
});

const envelopeSchema = z.object({
  error: z.object({ code: z.string(), message: z.string() }),
});

class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

interface ClientOptions {
  baseUrl: string;
  token: string;
  timeoutMs?: number;
  /** Injectable for tests; defaults to the global fetch. */
  fetch?: typeof fetch;
}

class TaskClient {
  readonly #options: ClientOptions;

  constructor(options: ClientOptions) {
    this.#options = options;
  }

  async #request<T>(
    method: string,
    path: string,
    schema: z.ZodType<T>,
    body?: unknown,
  ): Promise<T> {
    const doFetch = this.#options.fetch ?? fetch;
    const res = await doFetch(new URL(path, this.#options.baseUrl), {
      method,
      headers: {
        Accept: 'application/json',
        Authorization: `Bearer ${this.#options.token}`,
        ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(this.#options.timeoutMs ?? 5000),
    });
    const payload: unknown = await res.json();
    if (!res.ok) {
      const envelope = envelopeSchema.safeParse(payload);
      throw envelope.success
        ? new ApiError(res.status, envelope.data.error.code, envelope.data.error.message)
        : new ApiError(res.status, 'unknown', `unexpected error body (HTTP ${res.status})`);
    }
    return schema.parse(payload);
  }

  createTask(input: { title: string; tags?: string[] }): Promise<Task> {
    return this.#request('POST', '/tasks', taskSchema, input);
  }

  getTask(id: number): Promise<Task> {
    return this.#request('GET', `/tasks/${id}`, taskSchema);
  }

  listTasks(query: { status?: string; limit?: number } = {}): Promise<z.infer<typeof listSchema>> {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined) {
        params.set(key, String(value));
      }
    }
    return this.#request('GET', `/tasks?${params}`, listSchema);
  }
}

// A fake Task Tracker server: in memory, one valid token.
const tasks: Task[] = [];
const server = await startServer(async (req, res) => {
  const send = (status: number, payload: unknown) => {
    res.writeHead(status, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(payload));
  };
  const fail = (status: number, code: string, message: string) =>
    send(status, { error: { code, message } });

  if (req.headers.authorization !== 'Bearer s3cret') {
    return fail(401, 'unauthorized', 'missing or invalid token');
  }
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (req.method === 'POST' && url.pathname === '/tasks') {
    let raw = '';
    for await (const chunk of req) {
      raw += chunk;
    }
    const input = JSON.parse(raw) as { title: string; tags?: string[] };
    const task: Task = {
      id: tasks.length + 1,
      title: input.title,
      status: 'todo',
      due_date: null,
      tags: input.tags ?? [],
      owner_id: null,
      created_at: '2030-01-01T09:00:00Z',
    };
    tasks.push(task);
    return send(201, task);
  }
  const match = /^\/tasks\/(\d+)$/.exec(url.pathname);
  if (req.method === 'GET' && match?.[1] !== undefined) {
    const task = tasks.find((t) => t.id === Number(match[1]));
    return task ? send(200, task) : fail(404, 'not_found', 'task not found');
  }
  if (req.method === 'GET' && url.pathname === '/tasks') {
    return send(200, { items: tasks, total: tasks.length, limit: 20, offset: 0 });
  }
  return fail(404, 'not_found', 'no such route');
});

try {
  const client = new TaskClient({ baseUrl: server.url, token: 's3cret' });
  const created = await client.createTask({ title: 'Write docs', tags: ['docs'] });
  console.log('created', created);
  console.log('fetched', (await client.getTask(created.id)).title);
  console.log('list total', (await client.listTasks({ status: 'todo', limit: 10 })).total);

  for (const attempt of [
    () => client.getTask(42),
    () => new TaskClient({ baseUrl: server.url, token: 'wrong' }).getTask(1),
  ]) {
    try {
      await attempt();
    } catch (err) {
      if (err instanceof ApiError) {
        console.log(`ApiError ${err.status} ${err.code}: ${err.message}`);
      } else {
        throw err;
      }
    }
  }
} finally {
  await server.close();
}
