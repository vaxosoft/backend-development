/** A fixed-capacity cache that evicts the least recently used entry. */
export class LruCache<K, V> implements Iterable<[K, V]> {
  readonly #capacity: number;
  // A Map iterates in insertion order, so the first key is always the oldest.
  readonly #entries = new Map<K, V>();

  constructor(capacity: number) {
    if (!Number.isInteger(capacity) || capacity < 1) {
      throw new RangeError(`capacity must be a positive integer, got ${capacity}`);
    }
    this.#capacity = capacity;
  }

  get size(): number {
    return this.#entries.size;
  }

  /** Returns the value and marks the key as most recently used. */
  get(key: K): V | undefined {
    if (!this.#entries.has(key)) return undefined;
    const value = this.#entries.get(key) as V;
    this.#entries.delete(key);
    this.#entries.set(key, value);
    return value;
  }

  /** Returns the value without changing the recency order. */
  peek(key: K): V | undefined {
    return this.#entries.get(key);
  }

  has(key: K): boolean {
    return this.#entries.has(key);
  }

  /** Inserts or updates (making the key most recent); evicts the oldest entry when over capacity. */
  set(key: K, value: V): this {
    this.#entries.delete(key);
    this.#entries.set(key, value);
    if (this.#entries.size > this.#capacity) {
      const oldest = this.#entries.keys().next();
      if (!oldest.done) this.#entries.delete(oldest.value);
    }
    return this;
  }

  delete(key: K): boolean {
    return this.#entries.delete(key);
  }

  /** Iterates entries from least to most recently used. */
  [Symbol.iterator](): Iterator<[K, V]> {
    return this.#entries.entries();
  }
}

/** Wraps a single-argument function so repeated calls are answered from `cache`. */
export function memoize<A, R>(fn: (arg: A) => R, cache: LruCache<A, R>): (arg: A) => R {
  return (arg) => {
    if (cache.has(arg)) return cache.get(arg) as R;
    const result = fn(arg);
    cache.set(arg, result);
    return result;
  };
}
