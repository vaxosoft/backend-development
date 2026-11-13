/** Every parser throws this and names the offending variable, so the operator knows what to fix. */
export class ConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ConfigError';
  }
}

/**
 * `undefined` or `''` -> `fallback`.
 * `1`, `true`, `yes`, `on` -> true; `0`, `false`, `no`, `off` -> false (case-insensitive, surrounding
 * whitespace ignored). Anything else throws a `ConfigError` whose message contains `name`.
 */
export function parseBoolean(_name: string, _raw: string | undefined, _fallback: boolean): boolean {
  throw new Error('TODO');
}

/**
 * Strict base-10 integer (optional leading `-`): `"8080abc"`, `"2.5"`, `"1e3"` and `" "` are errors.
 * `undefined` or `''` -> `options.fallback`, or a `ConfigError` ("... is required") when there is none.
 * Values outside `min`/`max` (when given) throw a `ConfigError`. All messages contain `name`.
 */
export function parseInteger(
  _name: string,
  _raw: string | undefined,
  _options: { fallback?: number; min?: number; max?: number } = {},
): number {
  throw new Error('TODO');
}

/**
 * Durations with a mandatory unit: `500ms`, `2s`, `5m`, `1h` -> milliseconds.
 * A bare number (`"30"`) is ambiguous and therefore a `ConfigError`, as is any other text.
 * `undefined` or `''` -> `fallbackMs`. Messages contain `name`.
 */
export function parseDuration(
  _name: string,
  _raw: string | undefined,
  _fallbackMs: number,
): number {
  throw new Error('TODO');
}

/** `"docs, p1 ,,urgent"` -> `["docs", "p1", "urgent"]`; `undefined` -> `[]`. */
export function parseList(_raw: string | undefined): string[] {
  throw new Error('TODO');
}
