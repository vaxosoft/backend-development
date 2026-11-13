export const MAX_IDS = 50;

export type ParsedIds = { ok: true; ids: number[] } | { ok: false; message: string };

export function parseIds(raw: unknown): ParsedIds {
  if (typeof raw !== 'string' || raw.trim() === '') {
    return { ok: false, message: 'ids is required, for example ?ids=1,2,3' };
  }
  // split keeps empty entries, so "1,,2" is rejected below
  const parts = raw.split(',').map((part) => part.trim());
  if (parts.length > MAX_IDS) {
    return { ok: false, message: `at most ${MAX_IDS} ids are allowed` };
  }
  const ids = parts.map((part) => (/^\d+$/.test(part) ? Number(part) : Number.NaN));
  if (!ids.every(Number.isSafeInteger)) {
    return { ok: false, message: 'ids must be non-negative integers separated by commas' };
  }
  return { ok: true, ids };
}
