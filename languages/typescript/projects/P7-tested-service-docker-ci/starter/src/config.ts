export const LOG_LEVELS = ['debug', 'info', 'warn', 'error'] as const;
export const ENVIRONMENTS = ['development', 'test', 'production'] as const;

export type LogLevel = (typeof LOG_LEVELS)[number];
export type Environment = (typeof ENVIRONMENTS)[number];

export interface Config {
  host: string;
  port: number;
  logLevel: LogLevel;
  env: Environment;
}

export type ConfigResult = { ok: true; value: Config } | { ok: false; errors: string[] };

export const defaultConfig: Config = {
  host: '127.0.0.1',
  port: 8080,
  logLevel: 'info',
  env: 'development',
};

/**
 * TODO: build the config from environment variables and validate it.
 * - `ADDR` (`host:port`, default `127.0.0.1:8080`) sets host and port
 * - `PORT` overrides the port of `ADDR` (integer 0-65535)
 * - `LOG_LEVEL`: one of LOG_LEVELS (default `info`)
 * - `ENV`: one of ENVIRONMENTS (default `development`)
 * - empty strings count as unset
 * Report every problem at once (`errors`), each message naming the variable and the bad value.
 */
export function loadConfig(_env: Readonly<Record<string, string | undefined>>): ConfigResult {
  return { ok: true, value: { ...defaultConfig } };
}
