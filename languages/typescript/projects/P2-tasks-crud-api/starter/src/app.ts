import Fastify, { type FastifyInstance } from 'fastify';
import type { Config } from './config.ts';
import { openDatabase } from './db.ts';
import { registerErrorHandling } from './errors.ts';
import { SqliteTaskRepository } from './repository.ts';
import { registerTaskRoutes } from './routes.ts';

export async function buildApp(config: Config): Promise<FastifyInstance> {
  const app = Fastify({ logger: config.logger ?? false });

  const db = openDatabase(config.dbPath);
  app.addHook('onClose', async () => {
    db.close();
  });

  const repo = new SqliteTaskRepository(db, config.clock ?? (() => new Date()));
  registerErrorHandling(app);
  registerTaskRoutes(app, repo);
  return app;
}
