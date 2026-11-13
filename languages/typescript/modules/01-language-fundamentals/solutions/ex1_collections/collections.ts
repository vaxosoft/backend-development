/** Returns a new array sorted in ascending numeric order. The input must not be modified. */
export function sortedNumbers(numbers: readonly number[]): number[] {
  return numbers.toSorted((a, b) => a - b);
}

/**
 * Normalises tags: trim, lowercase, drop empty strings and duplicates.
 * The first occurrence of each tag keeps its position.
 */
export function uniqueTags(tags: readonly string[]): string[] {
  const seen = new Set<string>();
  for (const tag of tags) {
    const normalised = tag.trim().toLowerCase();
    if (normalised !== '') seen.add(normalised);
  }
  return [...seen];
}

/** Counts items per key, keeping keys in first-seen order. */
export function countBy<T>(items: readonly T[], key: (item: T) => string): Map<string, number> {
  const counts = new Map<string, number>();
  for (const item of items) {
    const k = key(item);
    counts.set(k, (counts.get(k) ?? 0) + 1);
  }
  return counts;
}
