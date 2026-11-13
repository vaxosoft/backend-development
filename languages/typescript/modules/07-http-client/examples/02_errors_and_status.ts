// fetch only rejects on network failures. A 404 or 500 is a *successful* fetch: check res.ok yourself.
import { startServer } from './_shared.ts';

class HttpError extends Error {
  readonly status: number;
  readonly body: string;

  constructor(status: number, body: string) {
    super(`HTTP ${status}`);
    this.name = 'HttpError';
    this.status = status;
    this.body = body;
  }
}

async function getJson(url: string): Promise<unknown> {
  const res = await fetch(url);
  if (!res.ok) {
    // Reading the body also releases the connection back to the pool.
    throw new HttpError(res.status, await res.text());
  }
  return res.json();
}

const server = await startServer((req, res) => {
  if (req.url === '/tasks/1') {
    res.setHeader('Content-Type', 'application/json');
    res.end('{"id":1,"title":"Write docs"}');
  } else if (req.url === '/boom') {
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end('{"error":{"code":"internal","message":"unexpected error"}}');
  } else {
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end('{"error":{"code":"not_found","message":"no such task"}}');
  }
});

try {
  const naive = await fetch(`${server.url}/tasks/99`);
  console.log('fetch resolved:', naive.status, naive.ok);
  await naive.body?.cancel();

  console.log(await getJson(`${server.url}/tasks/1`));

  for (const path of ['/tasks/99', '/boom']) {
    try {
      await getJson(server.url + path);
    } catch (err) {
      if (err instanceof HttpError) {
        console.log(path, '->', err.status, err.body);
      } else {
        throw err;
      }
    }
  }
} finally {
  await server.close();
}

// A network failure is the only case where fetch itself rejects, with a TypeError.
const closed = await startServer(() => {});
const deadUrl = closed.url;
await closed.close();
try {
  await fetch(deadUrl);
} catch (err) {
  if (err instanceof TypeError) {
    const cause = err.cause as NodeJS.ErrnoException;
    console.log(err.message, '/', cause.code);
  }
}
