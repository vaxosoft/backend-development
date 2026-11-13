/** A fixed-capacity cache that evicts the least recently used entry. */
export class LruCache<K, V> implements Iterable<[K, V]> {
  constructor(_capacity: number) {
    throw new Error('TODO');
  }

  get size(): number {
    throw new Error('TODO');
  }

  /** Returns the value and marks the key as most recently used. */
  get(_key: K): V | undefined {
    throw new Error('TODO');
  }

  /** Returns the value without changing the recency order. */
  peek(_key: K): V | undefined {
    throw new Error('TODO');
  }

  has(_key: K): boolean {
    throw new Error('TODO');
  }

  /** Inserts or updates (making the key most recent); evicts the oldest entry when over capacity. */
  set(_key: K, _value: V): this {
    throw new Error('TODO');
  }

  delete(_key: K): boolean {
    throw new Error('TODO');
  }

  /** Iterates entries from least to most recently used. */
  [Symbol.iterator](): Iterator<[K, V]> {
    throw new Error('TODO');
  }
}

/** Wraps a single-argument function so repeated calls are answered from `cache`. */
export function memoize<A, R>(_fn: (arg: A) => R, _cache: LruCache<A, R>): (arg: A) => R {
  throw new Error('TODO');
}
