export type Decision = { allowed: true } | { allowed: false; retryAfterSeconds: number };

/**
 * TODO(P5): a token bucket per key.
 *
 * - a new key starts with a full bucket of `burst` tokens
 * - `take(key)` removes one token and allows the call; with less than one token it denies
 * - tokens refill continuously at `refillPerSecond`, never above `burst`; time comes from `now()`
 *   (milliseconds), never from `Date.now()` directly
 * - when denied, `retryAfterSeconds` is the whole number of seconds (at least 1) until one token exists
 */
export class TokenBucketLimiter {
  readonly burst: number;
  readonly refillPerSecond: number;
  readonly now: () => number;

  constructor(burst: number, refillPerSecond: number, now: () => number) {
    this.burst = burst;
    this.refillPerSecond = refillPerSecond;
    this.now = now;
  }

  take(_key: string): Decision {
    throw new Error('TODO');
  }
}
