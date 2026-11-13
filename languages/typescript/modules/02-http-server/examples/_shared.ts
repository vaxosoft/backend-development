// Shared by the server examples. Not runnable on its own.

/** Reads `ADDR` (`host:port`, default `127.0.0.1:8080`). */
export function listenAddress(value: string | undefined = process.env.ADDR): {
  host: string;
  port: number;
} {
  const addr = value ?? '127.0.0.1:8080';
  const index = addr.lastIndexOf(':');
  const port = Number(addr.slice(index + 1));
  if (index < 1 || !Number.isInteger(port) || port < 0 || port > 65535) {
    throw new Error(`ADDR must look like host:port, got ${JSON.stringify(addr)}`);
  }
  return { host: addr.slice(0, index), port };
}

/** Runs `close` once on Ctrl-C or SIGTERM, then lets the process exit when the loop drains. */
export function closeOnSignal(close: () => Promise<void>): void {
  let closing = false;
  const handler = (signal: NodeJS.Signals): void => {
    if (closing) return;
    closing = true;
    console.log(`${signal} received, shutting down`);
    close().then(
      () => console.log('bye'),
      (error: unknown) => {
        console.error('shutdown failed', error);
        process.exitCode = 1;
      },
    );
  };
  process.on('SIGINT', handler);
  process.on('SIGTERM', handler);
}
