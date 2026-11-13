export const MAX_IDS = 50;

export type ParsedIds = { ok: true; ids: number[] } | { ok: false; message: string };

/**
 * TODO(P4): validate the raw `ids` query value ("1,2,3").
 *
 * - must be a string with at least one entry
 * - at most MAX_IDS entries
 * - every entry (after trimming spaces) is a non-negative integer: "1,a", "1,,2", "-1" and "1.5" are invalid
 * - keep the order of the entries
 */
export function parseIds(_raw: unknown): ParsedIds {
  throw new Error('TODO');
}
