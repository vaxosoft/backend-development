import { createApp, Metrics, memorySink, registerHealth, steppingClock } from './_shared.ts';

const metrics = new Metrics();
let databaseUp = true;

const app = createApp({
  sink: memorySink(),
  now: steppingClock(4),
  deterministic: true,
  onCompleted: (request) => metrics.record(request),
});

registerHealth(app, metrics, {
  database: async () => {
    if (!databaseUp) throw new Error('connection refused');
  },
});
app.get('/tasks/:id', async (req) => ({ id: Number((req.params as { id: string }).id) }));

for (const url of ['/healthz', '/readyz', '/tasks/1', '/tasks/2', '/tasks/3', '/nope']) {
  const res = await app.inject({ url });
  console.log(res.statusCode, url, res.body);
}

databaseUp = false;
const notReady = await app.inject({ url: '/readyz' });
console.log(notReady.statusCode, notReady.body);

console.log('--- /metrics');
console.log((await app.inject({ url: '/metrics' })).body);
await app.close();
