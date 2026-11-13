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

type Env = Readonly<Record<string, string | undefined>>;

function parsePort(raw: string): number | null {
  const port = /^\d+$/.test(raw) ? Number(raw) : Number.NaN;
  return port <= 65535 ? port : null;
}

function oneOf<T extends string>(allowed: readonly T[], raw: string): T | null {
  return allowed.find((candidate) => candidate === raw) ?? null;
}

/** Reads ADDR, PORT, LOG_LEVEL and ENV; reports every problem at once instead of stopping at the first. */
export function loadConfig(env: Env): ConfigResult {
  // Environment values are strings, and `FOO=` is an empty string: treat it as unset.
  const read = (name: string) => env[name]?.trim() || undefined;
  const errors: string[] = [];
  const value: Config = { ...defaultConfig };

  const addr = read('ADDR');
  if (addr !== undefined) {
    const split = addr.lastIndexOf(':');
    const port = parsePort(addr.slice(split + 1));
    if (split <= 0 || port === null) {
      errors.push(`ADDR must look like host:port, got "${addr}"`);
    } else {
      value.host = addr.slice(0, split);
      value.port = port;
    }
  }

  const rawPort = read('PORT');
  if (rawPort !== undefined) {
    const port = parsePort(rawPort);
    if (port === null) {
      errors.push(`PORT must be an integer from 0 to 65535, got "${rawPort}"`);
    } else {
      value.port = port;
    }
  }

  const rawLevel = read('LOG_LEVEL');
  if (rawLevel !== undefined) {
    const level = oneOf(LOG_LEVELS, rawLevel);
    if (level === null) {
      errors.push(`LOG_LEVEL must be one of ${LOG_LEVELS.join(', ')}, got "${rawLevel}"`);
    } else {
      value.logLevel = level;
    }
  }

  const rawEnv = read('ENV');
  if (rawEnv !== undefined) {
    const name = oneOf(ENVIRONMENTS, rawEnv);
    if (name === null) {
      errors.push(`ENV must be one of ${ENVIRONMENTS.join(', ')}, got "${rawEnv}"`);
    } else {
      value.env = name;
    }
  }

  return errors.length > 0 ? { ok: false, errors } : { ok: true, value };
}
