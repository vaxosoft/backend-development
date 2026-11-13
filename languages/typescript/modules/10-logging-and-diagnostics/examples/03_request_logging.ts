import { createApp, memorySink, steppingClock } from './_shared.ts';

const sink = memorySink();
const app = createApp({ sink, now: steppingClock(5), deterministic: true });

app.get('/tasks', async () => ({ items: [], total: 0 }));
app.get('/boom', async () => {
  throw new Error('connection to db:5432 refused');
});

const ok = await app.inject({ url: '/tasks?token=secret', headers: { 'x-request-id': 'req-123' } });
console.log(ok.statusCode, ok.headers['x-request-id']);

const boom = await app.inject({ url: '/boom' });
console.log(boom.statusCode, boom.body);

const hostile = await app.inject({
  url: '/tasks',
  headers: { 'x-request-id': 'bad id\twith spaces' },
});
console.log(hostile.statusCode, hostile.headers['x-request-id'] !== 'bad id\twith spaces');

const missing = await app.inject({ url: '/nope', headers: { 'x-request-id': 'req-404' } });
console.log(missing.statusCode, missing.body);

// Generated ids are random, so the demo masks them.
console.log('--- log lines');
for (const line of sink.lines) {
  console.log(line.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/, '<generated-uuid>'));
}
await app.close();
