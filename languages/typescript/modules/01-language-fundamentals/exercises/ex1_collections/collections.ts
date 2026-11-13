/** Returns a new array sorted in ascending numeric order. The input must not be modified. */
export function sortedNumbers(_numbers: readonly number[]): number[] {
  throw new Error('TODO');
}

/**
 * Normalises tags: trim, lowercase, drop empty strings and duplicates.
 * The first occurrence of each tag keeps its position.
 */
export function uniqueTags(_tags: readonly string[]): string[] {
  throw new Error('TODO');
}

/** Counts items per key, keeping keys in first-seen order. */
export function countBy<T>(_items: readonly T[], _key: (item: T) => string): Map<string, number> {
  throw new Error('TODO');
}
