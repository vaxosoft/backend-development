import pino, { type Logger } from 'pino';

export interface LogSink {
  write(line: string): void;
}

export interface LoggerOptions {
  service: string;
  /** Minimum level to emit. Defaults to `info`. */
  level?: string;
  sink: LogSink;
}

/**
 * Builds the service logger.
 *
 * - every line carries `service` and no `pid` / `hostname`
 * - secrets are replaced by `[REDACTED]`: `password`, `token`, `authorization` at the top level,
 *   `password` / `token` one level down (`user.password`), and the `authorization` / `cookie`
 *   headers under `req.headers`
 */
export function createLogger({ service, level = 'info', sink }: LoggerOptions): Logger {
  return pino(
    {
      level,
      base: { service },
      redact: {
        paths: [
          'password',
          'token',
          'authorization',
          '*.password',
          '*.token',
          'req.headers.authorization',
          'req.headers.cookie',
        ],
        censor: '[REDACTED]',
      },
    },
    sink,
  );
}
