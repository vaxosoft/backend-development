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
export function loadConfig(_env: Record<string, string | undefined>): Config {
  throw new Error('TODO');
}
