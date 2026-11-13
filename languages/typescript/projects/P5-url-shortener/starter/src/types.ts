import type { Config } from './config.ts';
import type { TokenBucketLimiter } from './rate-limit.ts';
import type { Store } from './store.ts';

declare module 'fastify' {
  interface FastifyRequest {
    /** Set by the authentication step; read by the request logger. */
    userId: number | null;
  }
}

export interface Deps {
  config: Config;
  store: Store;
  limiter: TokenBucketLimiter;
}
