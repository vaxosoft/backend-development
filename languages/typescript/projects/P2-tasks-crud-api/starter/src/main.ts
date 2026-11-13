import { buildApp } from './app.ts';

const addr = process.env.ADDR ?? '127.0.0.1:8080';
const separator = addr.lastIndexOf(':');
const host = addr.slice(0, separator);
const port = Number(addr.slice(separator + 1));

const app = await buildApp({ dbPath: process.env.DB_PATH ?? 'tasks.db', logger: true });

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.once(signal, () => {
    app.close().catch((error: unknown) => {
      console.error(error);
      process.exitCode = 1;
    });
  });
}

await app.listen({ host, port });
