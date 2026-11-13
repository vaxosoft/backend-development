import { createServer, type RequestListener } from 'node:http';

export interface LocalServer {
  url: string;
  close(): Promise<void>;
}

/** Starts a throw-away HTTP server on an ephemeral loopback port so examples need no network. */
export async function startServer(handler: RequestListener): Promise<LocalServer> {
  const server = createServer(handler);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (address === null || typeof address === 'string') {
    throw new Error('expected a TCP address');
  }
  return {
    url: `http://127.0.0.1:${address.port}`,
    close: () =>
      new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()));
        server.closeAllConnections();
      }),
  };
}
