import { z } from 'zod';

/** Anything with a `write(line)` method: stdout in production, an array collector in tests. */
export interface LogStream {
  write(line: string): void;
}

export interface Config {
  jwtSecret: string;
  /** SQLite file, or ':memory:' */
  dbPath: string;
  /** Token bucket capacity for POST /links, per user. */
  rateLimitBurst: number;
  rateLimitRefillPerSecond: number;
  /** scrypt N parameter; tests lower it so hashing is fast. */
  scryptCost: number;
  /** Clock for the rate limiter, in milliseconds. Injectable so tests need no sleeping. */
  now: () => number;
  logStream: LogStream;
}

const envSchema = z.object({
  JWT_SECRET: z.string().min(32, 'JWT_SECRET must be at least 32 characters'),
  DB_PATH: z.string().default(':memory:'),
  RATE_LIMIT_BURST: z.coerce.number().int().min(1).default(5),
  RATE_LIMIT_REFILL_PER_SECOND: z.coerce.number().positive().default(1),
  SCRYPT_COST: z.coerce.number().int().min(1024).default(16384),
});

/** Environment values are strings: validate and convert once, fail fast at startup. */
export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  const e = envSchema.parse(env);
  return {
    jwtSecret: e.JWT_SECRET,
    dbPath: e.DB_PATH,
    rateLimitBurst: e.RATE_LIMIT_BURST,
    rateLimitRefillPerSecond: e.RATE_LIMIT_REFILL_PER_SECOND,
    scryptCost: e.SCRYPT_COST,
    now: Date.now,
    logStream: process.stdout,
  };
}
