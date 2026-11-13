import { createServer, type RequestListener, type Server } from 'node:http';

export interface ServerOptions {
  /** Default `127.0.0.1`. */
  host?: string;
  /** Default `0` (let the OS pick a free port). */
  port?: number;
  /** Default 30000. Maximum time to receive the whole request. */
  requestTimeoutMs?: number;
  /** Default 10000. Maximum time to receive the request headers. */
  headersTimeoutMs?: number;
  /** Default 5000. How long an idle keep-alive socket stays open. */
  keepAliveTimeoutMs?: number;
}

export interface RunningServer {
  readonly server: Server;
  /** The port the server is actually listening on. */
  readonly port: number;
  /**
   * Graceful shutdown: stop accepting new connections, let in-flight requests finish,
   * close idle keep-alive sockets, and resolve once everything is closed.
   * If connections are still open after `graceMs` (default 10000), destroy them.
   * Calling it again returns the same promise.
   */
  shutdown(graceMs?: number): Promise<void>;
}

/** Creates a `node:http` server, applies the timeouts, and resolves once it is listening. */
export async function startServer(
  handler: RequestListener,
  options: ServerOptions = {},
): Promise<RunningServer> {
  const server = createServer(handler);
  server.headersTimeout = options.headersTimeoutMs ?? 10_000;
  server.requestTimeout = options.requestTimeoutMs ?? 30_000;
  server.keepAliveTimeout = options.keepAliveTimeoutMs ?? 5_000;

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(options.port ?? 0, options.host ?? '127.0.0.1', () => {
      server.off('error', reject);
      resolve();
    });
  });

  const address = server.address();
  if (address === null || typeof address === 'string') {
    throw new Error('server is not listening on a TCP port');
  }

  let stopping: Promise<void> | undefined;
  // A connection that was busy when shutdown began turns idle after its response: close it then.
  server.on('request', (req, res) => {
    res.once('finish', () => {
      if (stopping) req.socket.end();
    });
  });
  const shutdown = (graceMs = 10_000): Promise<void> => {
    stopping ??= new Promise<void>((resolve, reject) => {
      const deadline = setTimeout(() => server.closeAllConnections(), graceMs);
      server.close((error) => {
        clearTimeout(deadline);
        if (error) reject(error);
        else resolve();
      });
      server.closeIdleConnections();
    });
    return stopping;
  };

  return { server, port: address.port, shutdown };
}
