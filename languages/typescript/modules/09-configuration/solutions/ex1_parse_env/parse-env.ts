/** Every parser throws this and names the offending variable, so the operator knows what to fix. */
export class ConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ConfigError';
  }
}

const TRUE_WORDS = new Set(['1', 'true', 'yes', 'on']);
const FALSE_WORDS = new Set(['0', 'false', 'no', 'off']);

function isUnset(raw: string | undefined): raw is undefined {
  return raw === undefined || raw === '';
}

/**
 * `undefined` or `''` -> `fallback`.
 * `1`, `true`, `yes`, `on` -> true; `0`, `false`, `no`, `off` -> false (case-insensitive, surrounding
 * whitespace ignored). Anything else throws a `ConfigError` whose message contains `name`.
 */
export function parseBoolean(name: string, raw: string | undefined, fallback: boolean): boolean {
  if (isUnset(raw)) {
    return fallback;
  }
  const word = raw.trim().toLowerCase();
  if (TRUE_WORDS.has(word)) {
    return true;
  }
  if (FALSE_WORDS.has(word)) {
    return false;
  }
  throw new ConfigError(`${name}: expected a boolean (true/false/1/0/yes/no/on/off), got "${raw}"`);
}

/**
 * Strict base-10 integer (optional leading `-`): `"8080abc"`, `"2.5"`, `"1e3"` and `" "` are errors.
 * `undefined` or `''` -> `options.fallback`, or a `ConfigError` ("... is required") when there is none.
 * Values outside `min`/`max` (when given) throw a `ConfigError`. All messages contain `name`.
 */
export function parseInteger(
  name: string,
  raw: string | undefined,
  options: { fallback?: number; min?: number; max?: number } = {},
): number {
  if (isUnset(raw)) {
    if (options.fallback === undefined) {
      throw new ConfigError(`${name} is required`);
    }
    return options.fallback;
  }
  if (!/^-?\d+$/.test(raw.trim())) {
    throw new ConfigError(`${name}: expected an integer, got "${raw}"`);
  }
  const value = Number(raw.trim());
  const { min = Number.NEGATIVE_INFINITY, max = Number.POSITIVE_INFINITY } = options;
  if (value < min || value > max) {
    throw new ConfigError(`${name}: ${value} is outside ${min}..${max}`);
  }
  return value;
}

const UNIT_MS = { ms: 1, s: 1000, m: 60_000, h: 3_600_000 } as const;

/**
 * Durations with a mandatory unit: `500ms`, `2s`, `5m`, `1h` -> milliseconds.
 * A bare number (`"30"`) is ambiguous and therefore a `ConfigError`, as is any other text.
 * `undefined` or `''` -> `fallbackMs`. Messages contain `name`.
 */
export function parseDuration(name: string, raw: string | undefined, fallbackMs: number): number {
  if (isUnset(raw)) {
    return fallbackMs;
  }
  const match = /^(\d+)(ms|s|m|h)$/.exec(raw.trim());
  if (match === null) {
    throw new ConfigError(`${name}: expected a duration like 500ms, 2s, 5m or 1h, got "${raw}"`);
  }
  const unit = match[2] as keyof typeof UNIT_MS;
  return Number(match[1]) * UNIT_MS[unit];
}

/** `"docs, p1 ,,urgent"` -> `["docs", "p1", "urgent"]`; `undefined` -> `[]`. */
export function parseList(raw: string | undefined): string[] {
  return (raw ?? '')
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== '');
}
