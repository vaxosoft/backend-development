// Manual dependency injection: config is parsed once, a composition root wires the parts,
// and everything else receives its dependencies as constructor/function arguments.
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import Fastify, { type FastifyInstance } from 'fastify';

interface Task {
  id: number;
  title: string;
  status: 'todo';
  created_at: string;
}

interface Clock {
  now(): Date;
}
interface Logger {
  info(message: string): void;
}
interface TaskStore {
  add(task: Task): Promise<void>;
  list(): Promise<Task[]>;
}

class MemoryTaskStore implements TaskStore {
  readonly #tasks: Task[] = [];

  async add(task: Task): Promise<void> {
    this.#tasks.push(task);
  }

  async list(): Promise<Task[]> {
    return [...this.#tasks];
  }
}

class JsonFileTaskStore implements TaskStore {
  readonly #file: string;

  constructor(file: string) {
    this.#file = file;
  }

  async add(task: Task): Promise<void> {
    await writeFile(this.#file, JSON.stringify([...(await this.list()), task]));
  }

  async list(): Promise<Task[]> {
    try {
      return JSON.parse(await readFile(this.#file, 'utf8')) as Task[];
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
        return [];
      }
      throw err;
    }
  }
}

class TaskService {
  readonly #store: TaskStore;
  readonly #clock: Clock;
  readonly #logger: Logger;

  constructor(deps: { store: TaskStore; clock: Clock; logger: Logger }) {
    this.#store = deps.store;
    this.#clock = deps.clock;
    this.#logger = deps.logger;
  }

  async create(title: string): Promise<Task> {
    const existing = await this.#store.list();
    const task: Task = {
      id: existing.length + 1,
      title,
      status: 'todo',
      created_at: this.#clock.now().toISOString().replace('.000Z', 'Z'),
    };
    await this.#store.add(task);
    this.#logger.info(`created task ${task.id}`);
    return task;
  }

  list(): Promise<Task[]> {
    return this.#store.list();
  }
}

interface Config {
  store: 'memory' | 'file';
  dataFile: string;
  apiToken: string;
}

/** Test doubles (or other environments) replace parts here; production passes nothing. */
interface Overrides {
  clock?: Clock;
  logger?: Logger;
  store?: TaskStore;
}

// The composition root: the only place that knows which concrete classes are used.
function buildApp(config: Config, overrides: Overrides = {}): FastifyInstance {
  const store =
    overrides.store ??
    (config.store === 'file' ? new JsonFileTaskStore(config.dataFile) : new MemoryTaskStore());
  const service = new TaskService({
    store,
    clock: overrides.clock ?? { now: () => new Date() },
    logger: overrides.logger ?? console,
  });

  const app = Fastify();
  app.post<{ Body: { title: string } }>('/tasks', async (req, reply) =>
    reply.code(201).send(await service.create(req.body.title)),
  );
  app.get('/tasks', async () => ({ items: await service.list() }));
  app.get('/config', async () => ({ ...config, apiToken: '[REDACTED]' }));
  return app;
}

const logLines: string[] = [];
const fixedClock: Clock = { now: () => new Date('2030-01-01T09:00:00Z') };
const logger: Logger = { info: (message) => logLines.push(message) };

const dir = await mkdtemp(join(tmpdir(), 'm09-'));
try {
  const configs: Config[] = [
    { store: 'memory', dataFile: '', apiToken: 'secret-1' },
    { store: 'file', dataFile: join(dir, 'tasks.json'), apiToken: 'secret-2' },
  ];
  for (const config of configs) {
    const app = buildApp(config, { clock: fixedClock, logger });
    const created = await app.inject({
      method: 'POST',
      url: '/tasks',
      payload: { title: 'Write docs' },
    });
    console.log(config.store, created.statusCode, created.json());
    console.log('  list  :', (await app.inject('/tasks')).json().items.length);
    console.log('  config:', (await app.inject('/config')).json().apiToken);
    await app.close();
  }
  console.log('logged:', logLines);
  console.log(
    'file store persisted:',
    JSON.parse(await readFile(join(dir, 'tasks.json'), 'utf8')).length,
  );
} finally {
  await rm(dir, { recursive: true, force: true });
}
