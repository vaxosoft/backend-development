// Helpers shared by examples 01 and 05. Not runnable.
import { randomBytes, scrypt, timingSafeEqual } from 'node:crypto';

const KEY_LENGTH = 32;
const DEFAULT_COST = 16384;

function derive(password: string, salt: Buffer, cost: number): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    // NFKC so that visually identical passwords hash identically.
    scrypt(password.normalize('NFKC'), salt, KEY_LENGTH, { N: cost }, (err, key) => {
      if (err) reject(err);
      else resolve(key);
    });
  });
}

/** `scrypt$<cost>$<salt>$<hash>`: the cost travels with the hash so it can be raised later. */
export async function hashPassword(password: string, cost = DEFAULT_COST): Promise<string> {
  const salt = randomBytes(16);
  const key = await derive(password, salt, cost);
  return `scrypt$${cost}$${salt.toString('base64url')}$${key.toString('base64url')}`;
}

export async function verifyPassword(password: string, stored: string): Promise<boolean> {
  const [scheme, costText, saltText, keyText, ...rest] = stored.split('$');
  const cost = Number(costText);
  if (scheme !== 'scrypt' || rest.length > 0 || !Number.isInteger(cost) || !saltText || !keyText) {
    return false;
  }
  const expected = Buffer.from(keyText, 'base64url');
  const actual = await derive(password, Buffer.from(saltText, 'base64url'), cost);
  // timingSafeEqual throws when lengths differ, so guard first.
  return expected.length === actual.length && timingSafeEqual(expected, actual);
}

export type Take = { allowed: true } | { allowed: false; retryAfterSeconds: number };

export interface BucketOptions {
  burst: number;
  refillPerSecond: number;
  /** Clock in milliseconds; injectable for tests. */
  now?: () => number;
}

/** A token bucket per key: `burst` requests at once, then `refillPerSecond` on average. */
export class TokenBucketLimiter {
  readonly #burst: number;
  readonly #refillPerMs: number;
  readonly #now: () => number;
  readonly #buckets = new Map<string, { tokens: number; updatedAt: number }>();

  constructor({ burst, refillPerSecond, now = Date.now }: BucketOptions) {
    this.#burst = burst;
    this.#refillPerMs = refillPerSecond / 1000;
    this.#now = now;
  }

  take(key: string): Take {
    const t = this.#now();
    const bucket = this.#buckets.get(key) ?? { tokens: this.#burst, updatedAt: t };
    bucket.tokens = Math.min(
      this.#burst,
      bucket.tokens + (t - bucket.updatedAt) * this.#refillPerMs,
    );
    bucket.updatedAt = t;
    this.#buckets.set(key, bucket);

    if (bucket.tokens >= 1) {
      bucket.tokens -= 1;
      return { allowed: true };
    }
    return {
      allowed: false,
      retryAfterSeconds: Math.ceil((1 - bucket.tokens) / this.#refillPerMs / 1000),
    };
  }
}
