import { parseArgs } from 'node:util';
import { z } from 'zod';

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

const layerSchema = z
  .object({
    port: z.coerce.number().int().min(1).max(65535),
    logLevel: z.enum(['debug', 'info', 'warn', 'error']),
    dataDir: z.string().min(1),
    apiKey: z.string().min(1),
  })
  .partial()
  .strict();
type Layer = z.infer<typeof layerSchema>;

function validate(source: Exclude<Source, 'default'>, raw: unknown): Layer {
  const result = layerSchema.safeParse(raw);
  if (!result.success) {
    const problems = result.error.issues.map((issue) => {
      const keys =
        issue.code === 'unrecognized_keys' ? issue.keys.join(', ') : issue.path.join('.');
      return `${keys} (${source}): ${issue.message}`;
    });
    throw new ConfigError(problems.join('; '));
  }
  return result.data;
}

function fileLayer(text: string | undefined): Layer {
  if (text === undefined) {
    return {};
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (err) {
    throw new ConfigError('config file is not valid JSON', { cause: err });
  }
  return validate('file', parsed);
}

function envLayer(env: Record<string, string | undefined>): Layer {
  const pick = (name: string) => (env[name] === '' ? undefined : env[name]);
  return validate('env', {
    port: pick('APP_PORT'),
    logLevel: pick('APP_LOG_LEVEL'),
    dataDir: pick('APP_DATA_DIR'),
    apiKey: pick('APP_API_KEY'),
  });
}

function flagLayer(argv: string[]): Layer {
  try {
    const { values } = parseArgs({
      args: argv,
      options: {
        port: { type: 'string', short: 'p' },
        'log-level': { type: 'string' },
        'data-dir': { type: 'string' },
        'api-key': { type: 'string' },
      },
      strict: true,
    });
    return validate('flag', {
      port: values.port,
      logLevel: values['log-level'],
      dataDir: values['data-dir'],
      apiKey: values['api-key'],
    });
  } catch (err) {
    if (err instanceof ConfigError) {
      throw err;
    }
    throw new ConfigError(`invalid command line: ${(err as Error).message}`, { cause: err });
  }
}

/**
 * Layers the configuration: defaults < file < env < flags (later wins, per key).
 * - an env var set to `''` counts as unset
 * - unknown keys in the file, unknown flags and malformed JSON throw a `ConfigError`
 * - invalid values (port not an integer 1-65535, unknown log level) throw a `ConfigError` whose
 *   message names the key and the layer, e.g. `port (env): ...`
 * - `sources` says which layer each final value came from (`default` when nothing overrode it)
 */
export function resolveSettings(input: ResolveInput): Resolved {
  const layers: Array<[Exclude<Source, 'default'>, Layer]> = [
    ['file', fileLayer(input.file)],
    ['env', envLayer(input.env)],
    ['flag', flagLayer(input.argv)],
  ];
  const settings: Settings = { ...DEFAULTS };
  const sources: Resolved['sources'] = {
    port: 'default',
    logLevel: 'default',
    dataDir: 'default',
    apiKey: 'default',
  };
  for (const [source, layer] of layers) {
    for (const key of Object.keys(layer) as Array<keyof Layer>) {
      if (layer[key] !== undefined) {
        Object.assign(settings, { [key]: layer[key] });
        sources[key] = source;
      }
    }
  }
  return { settings, sources };
}

/** A copy that is safe to log: `apiKey` becomes `'[REDACTED]'` when set and stays `undefined` otherwise. */
export function redact(settings: Settings): Record<string, unknown> {
  return { ...settings, apiKey: settings.apiKey === undefined ? undefined : '[REDACTED]' };
}
