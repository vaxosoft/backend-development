export type Decision = { allowed: true } | { allowed: false; retryAfterSeconds: number };

interface Bucket {
  tokens: number;
  updatedAt: number;
}

export class TokenBucketLimiter {
  readonly burst: number;
  readonly refillPerSecond: number;
  readonly now: () => number;
  // Grows with the number of distinct users; a real service would evict idle buckets.
  private readonly buckets = new Map<string, Bucket>();

  constructor(burst: number, refillPerSecond: number, now: () => number) {
    this.burst = burst;
    this.refillPerSecond = refillPerSecond;
    this.now = now;
  }

  take(key: string): Decision {
    const now = this.now();
    const bucket = this.buckets.get(key) ?? { tokens: this.burst, updatedAt: now };
    const elapsedSeconds = Math.max(0, now - bucket.updatedAt) / 1000;
    bucket.tokens = Math.min(this.burst, bucket.tokens + elapsedSeconds * this.refillPerSecond);
    bucket.updatedAt = now;
    this.buckets.set(key, bucket);

    if (bucket.tokens >= 1) {
      bucket.tokens -= 1;
      return { allowed: true };
    }
    const retryAfterSeconds = Math.max(1, Math.ceil((1 - bucket.tokens) / this.refillPerSecond));
    return { allowed: false, retryAfterSeconds };
  }
}
