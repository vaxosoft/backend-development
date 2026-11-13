import { z } from 'zod';

export interface Config {
  upstreamBaseUrl: string;
  maxConcurrency: number;
  callTimeoutMs: number;
  maxRetries: number;
  backoffBaseMs: number;
  overallTimeoutMs: number;
}

export const defaultConfig: Config = {
  upstreamBaseUrl: 'http://127.0.0.1:9000',
  maxConcurrency: 4,
  callTimeoutMs: 2000,
  maxRetries: 2,
  backoffBaseMs: 100,
  overallTimeoutMs: 10_000,
};

const count = (min: number) => z.coerce.number().int().min(min);

const envSchema = z.object({
  UPSTREAM_BASE_URL: z.url().default(defaultConfig.upstreamBaseUrl),
  MAX_CONCURRENCY: count(1).default(defaultConfig.maxConcurrency),
  CALL_TIMEOUT_MS: count(1).default(defaultConfig.callTimeoutMs),
  MAX_RETRIES: count(0).default(defaultConfig.maxRetries),
  BACKOFF_BASE_MS: count(0).default(defaultConfig.backoffBaseMs),
  OVERALL_TIMEOUT_MS: count(1).default(defaultConfig.overallTimeoutMs),
});

/** Environment values are strings, so they are validated and converted once, at startup. */
export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  const e = envSchema.parse(env);
  return {
    upstreamBaseUrl: e.UPSTREAM_BASE_URL,
    maxConcurrency: e.MAX_CONCURRENCY,
    callTimeoutMs: e.CALL_TIMEOUT_MS,
    maxRetries: e.MAX_RETRIES,
    backoffBaseMs: e.BACKOFF_BASE_MS,
    overallTimeoutMs: e.OVERALL_TIMEOUT_MS,
  };
}
