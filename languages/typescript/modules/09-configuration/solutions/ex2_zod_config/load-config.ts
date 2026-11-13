import { z } from 'zod';

export class ConfigError extends Error {
  /** One entry per problem, formatted `VARIABLE: message`. */
  readonly issues: string[];

  constructor(issues: string[]) {
    super(`invalid configuration:\n${issues.map((i) => `  ${i}`).join('\n')}`);
    this.name = 'ConfigError';
    this.issues = issues;
  }
}

export interface Config {
  port: number;
  logLevel: 'debug' | 'info' | 'warn' | 'error';
  environment: 'development' | 'production' | 'test';
  databaseUrl: string;
  debug: boolean;
  allowedOrigins: string[];
  apiToken: string | undefined;
}

const csv = z.string().transform((raw) =>
  raw
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== ''),
);

const envSchema = z
  .object({
    PORT: z.coerce.number().int().min(1).max(65535).default(8080),
    LOG_LEVEL: z.enum(['debug', 'info', 'warn', 'error']).default('info'),
    ENV: z.enum(['development', 'production', 'test']).default('development'),
    DATABASE_URL: z.url(),
    DEBUG: z.stringbool().default(false),
    ALLOWED_ORIGINS: csv.default([]),
    API_TOKEN: z.string().min(16).optional(),
  })
  .refine((env) => env.ENV !== 'production' || env.API_TOKEN !== undefined, {
    path: ['API_TOKEN'],
    message: 'required when ENV=production',
  });

/**
 * Builds a `Config` from environment-style input using a zod schema.
 *
 * | variable          | rule                                                                   |
 * |-------------------|------------------------------------------------------------------------|
 * | `PORT`            | integer 1-65535, default 8080                                          |
 * | `LOG_LEVEL`       | debug, info, warn or error, default info                               |
 * | `ENV`             | development, production or test, default development                   |
 * | `DATABASE_URL`    | required, must be a URL                                                |
 * | `DEBUG`           | true/false words (`z.stringbool()`), default false                     |
 * | `ALLOWED_ORIGINS` | comma separated list (trimmed, no empty items), default empty          |
 * | `API_TOKEN`       | optional, at least 16 characters when present; REQUIRED when ENV=production |
 *
 * An empty string counts as "not set". Report *all* problems at once: throw a `ConfigError` whose
 * `issues` are `"VARIABLE: message"`. Never put the value of `API_TOKEN` into a message.
 */
export function loadConfig(env: Record<string, string | undefined>): Config {
  const present = Object.fromEntries(Object.entries(env).filter(([, value]) => value !== ''));
  const result = envSchema.safeParse(present);
  if (!result.success) {
    throw new ConfigError(
      result.error.issues.map((issue) => `${issue.path.join('.')}: ${issue.message}`),
    );
  }
  const parsed = result.data;
  return {
    port: parsed.PORT,
    logLevel: parsed.LOG_LEVEL,
    environment: parsed.ENV,
    databaseUrl: parsed.DATABASE_URL,
    debug: parsed.DEBUG,
    allowedOrigins: parsed.ALLOWED_ORIGINS,
    apiToken: parsed.API_TOKEN,
  };
}
