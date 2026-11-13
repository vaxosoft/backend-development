import type { RequestListener, Server } from 'node:http';

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
  _handler: RequestListener,
  _options: ServerOptions = {},
): Promise<RunningServer> {
  throw new Error('TODO');
}
