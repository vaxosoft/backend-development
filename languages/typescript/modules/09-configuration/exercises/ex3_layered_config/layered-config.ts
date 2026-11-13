export class ConfigError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'ConfigError';
  }
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface Settings {
  port: number;
  logLevel: LogLevel;
  dataDir: string;
  apiKey: string | undefined;
}

export type Source = 'default' | 'file' | 'env' | 'flag';

export interface ResolveInput {
  /** Text of a JSON config file (keys are the `Settings` names), or undefined when there is none. */
  file?: string;
  /** Environment: `APP_PORT`, `APP_LOG_LEVEL`, `APP_DATA_DIR`, `APP_API_KEY`. */
  env: Record<string, string | undefined>;
  /** Command line: `--port`/`-p`, `--log-level`, `--data-dir`, `--api-key` (each takes a value). */
  argv: string[];
}

export interface Resolved {
  settings: Settings;
  /** Which layer produced each final value. */
  sources: Record<keyof Settings, Source>;
}

export const DEFAULTS: Settings = {
  port: 8080,
  logLevel: 'info',
  dataDir: './data',
  apiKey: undefined,
};

/**
 * Layers the configuration: defaults < file < env < flags (later wins, per key).
 * - an env var set to `''` counts as unset
 * - unknown keys in the file, unknown flags and malformed JSON throw a `ConfigError`
 * - invalid values (port not an integer 1-65535, unknown log level) throw a `ConfigError` whose
 *   message names the key and the layer, e.g. `port (env): ...`
 * - `sources` says which layer each final value came from (`default` when nothing overrode it)
 */
export function resolveSettings(_input: ResolveInput): Resolved {
  throw new Error('TODO');
}

/** A copy that is safe to log: `apiKey` becomes `'[REDACTED]'` when set and stays `undefined` otherwise. */
export function redact(_settings: Settings): Record<string, unknown> {
  throw new Error('TODO');
}
