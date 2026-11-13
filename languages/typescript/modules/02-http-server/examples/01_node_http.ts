// The raw `node:http` server: no router, you parse the URL and write the response yourself.
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { closeOnSignal, listenAddress } from './_shared.ts';

function sendText(res: ServerResponse, status: number, body: string): void {
  res.writeHead(status, {
    'content-type': 'text/plain; charset=utf-8',
    'content-length': Buffer.byteLength(body),
  });
  res.end(body);
}

async function readBody(req: IncomingMessage, limit = 1024): Promise<string> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    size += chunk.length;
    if (size > limit) throw new RangeError('body too large');
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString('utf8');
}

const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url ?? '/', 'http://localhost');
    const hello = /^\/hello\/([^/]+)$/.exec(url.pathname);

    if (req.method === 'GET' && url.pathname === '/') {
      sendText(res, 200, 'node:http says hi\n');
    } else if (req.method === 'GET' && hello?.[1] !== undefined) {
      const name = decodeURIComponent(hello[1]);
      const shout = url.searchParams.get('shout') === 'true';
      sendText(res, 200, shout ? `HELLO, ${name.toUpperCase()}!\n` : `Hello, ${name}!\n`);
    } else if (req.method === 'POST' && url.pathname === '/echo') {
      sendText(res, 200, await readBody(req));
    } else if (url.pathname === '/' || url.pathname === '/echo') {
      res.setHeader('allow', url.pathname === '/' ? 'GET' : 'POST');
      sendText(res, 405, 'method not allowed\n');
    } else {
      sendText(res, 404, 'not found\n');
    }
  } catch (error) {
    // Async handlers must catch their own errors: node:http will not do it for you.
    if (error instanceof RangeError) return sendText(res, 413, 'payload too large\n');
    console.error(error);
    sendText(res, 500, 'internal error\n');
  }
});

// Timeouts protect the server from slow or silent clients.
server.requestTimeout = 30_000; // whole request must arrive within 30 s
server.headersTimeout = 10_000;
server.keepAliveTimeout = 5_000;

const { host, port } = listenAddress();
server.listen(port, host, () => console.log('listening'));

closeOnSignal(
  () =>
    new Promise((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve())); // stop accepting, let in-flight finish
      server.closeIdleConnections(); // drop idle keep-alive sockets so close() can complete
    }),
);
