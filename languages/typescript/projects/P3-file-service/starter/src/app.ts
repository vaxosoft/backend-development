import multipart from '@fastify/multipart';
import Fastify, { type FastifyInstance } from 'fastify';
import { type Config, DEFAULT_MAX_FILE_SIZE } from './config.ts';
import { registerErrorHandling } from './errors.ts';
import { registerFileRoutes } from './routes.ts';
import { FileStore } from './store.ts';

export async function buildApp(config: Config): Promise<FastifyInstance> {
  const app = Fastify({ logger: false });

  await app.register(multipart, {
    // Past the limit the file stream is truncated and emits 'limit'; the route must react to it.
    limits: { fileSize: config.maxFileSize ?? DEFAULT_MAX_FILE_SIZE, files: 1 },
  });

  const store = new FileStore(config.storageDir);
  await store.init();

  registerErrorHandling(app);
  registerFileRoutes(app, store);
  return app;
}
