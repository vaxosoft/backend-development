import { buildApp } from './app.ts';
import { parseAddr } from './config.ts';
import { run } from './server.ts';

const { host, port } = parseAddr(process.env.ADDR ?? '127.0.0.1:8080');
const app = await buildApp();

const shutdown = new AbortController();
for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.once(signal, () => shutdown.abort());
}

const server = await run(app, { host, port, graceMs: 10_000 }, shutdown.signal);
console.log(`listening on http://${host}:${server.port}`);
await server.stopped;
console.log('shut down cleanly');
