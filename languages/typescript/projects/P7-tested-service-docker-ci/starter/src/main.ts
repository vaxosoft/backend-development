import { buildApp } from './app.ts';
import { loadConfig } from './config.ts';

const loaded = loadConfig(process.env);
if (!loaded.ok) {
  console.error(`invalid configuration:\n${loaded.errors.map((e) => `  - ${e}`).join('\n')}`);
  process.exit(1);
}
const config = loaded.value;

const app = await buildApp({ logLevel: config.logLevel });

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.once(signal, () => {
    app.readiness.markNotReady();
    app.log.info({ signal }, 'shutting down');
    app.close().then(
      () => process.exit(0),
      () => process.exit(1),
    );
  });
}

await app.listen({ host: config.host, port: config.port });
app.readiness.markReady();
app.log.info({ env: config.env }, 'ready');
