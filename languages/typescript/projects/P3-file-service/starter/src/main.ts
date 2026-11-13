import { buildApp } from './app.ts';
import { DEFAULT_MAX_FILE_SIZE } from './config.ts';

const addr = process.env.ADDR ?? '127.0.0.1:8080';
const separator = addr.lastIndexOf(':');
const host = addr.slice(0, separator);
const port = Number(addr.slice(separator + 1));

const app = await buildApp({
  storageDir: process.env.STORAGE_DIR ?? './data',
  maxFileSize: Number(process.env.MAX_FILE_SIZE ?? DEFAULT_MAX_FILE_SIZE),
});

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.once(signal, () => {
    app.close().catch((error: unknown) => {
      console.error(error);
      process.exitCode = 1;
    });
  });
}

await app.listen({ host, port });
console.log(`listening on http://${host}:${port}`);
