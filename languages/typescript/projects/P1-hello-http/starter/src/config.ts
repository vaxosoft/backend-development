export interface Config {
  /** Receives one JSON line per finished request. */
  logSink: (line: string) => void;
  /** Receives the real error behind every 500 response. */
  errorSink: (error: unknown) => void;
  requestTimeoutMs: number;
  headersTimeoutMs: number;
  keepAliveTimeoutMs: number;
}

export function withDefaults(partial: Partial<Config> = {}): Config {
  return {
    logSink: (line) => console.log(line),
    errorSink: (error) => console.error(error),
    requestTimeoutMs: 10_000,
    headersTimeoutMs: 5_000,
    keepAliveTimeoutMs: 5_000,
    ...partial,
  };
}

export interface ListenAddress {
  host: string;
  port: number;
}

/** Parses `host:port`, the format of the ADDR environment variable. */
export function parseAddr(addr: string): ListenAddress {
  const separator = addr.lastIndexOf(':');
  const port = Number(addr.slice(separator + 1));
  if (separator < 1 || !Number.isInteger(port) || port < 0 || port > 65535) {
    throw new Error(`invalid ADDR "${addr}": want host:port`);
  }
  return { host: addr.slice(0, separator), port };
}
