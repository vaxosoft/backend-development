// Layered configuration: defaults < config file < .env file < environment < command-line flags.
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { parseArgs, parseEnv } from 'node:util';
import { z } from 'zod';

const settingsSchema = z.object({
  port: z.coerce.number().int().min(1).max(65535),
  logLevel: z.enum(['debug', 'info', 'warn', 'error']),
  dataDir: z.string().min(1),
});
type Settings = z.infer<typeof settingsSchema>;

// Every layer is partial and strict: a typo such as "prot" is an error, not silently ignored.
const layerSchema = settingsSchema.partial().strict();
type Layer = z.infer<typeof layerSchema>;

const DEFAULTS: Settings = { port: 8080, logLevel: 'info', dataDir: './data' };

function envLayer(env: Record<string, string | undefined>): Layer {
  const layer: Record<string, string> = {};
  if (env.APP_PORT !== undefined) layer.port = env.APP_PORT;
  if (env.APP_LOG_LEVEL !== undefined) layer.logLevel = env.APP_LOG_LEVEL;
  if (env.APP_DATA_DIR !== undefined) layer.dataDir = env.APP_DATA_DIR;
  return layerSchema.parse(layer);
}

function flagLayer(argv: string[]): Layer {
  const { values } = parseArgs({
    args: argv,
    options: {
      port: { type: 'string', short: 'p' },
      'log-level': { type: 'string' },
      'data-dir': { type: 'string' },
    },
    strict: true,
  });
  return layerSchema.parse({
    port: values.port,
    logLevel: values['log-level'],
    dataDir: values['data-dir'],
  });
}

function merge(layers: Array<[name: string, layer: Layer]>): {
  settings: Settings;
  sources: Record<string, string>;
} {
  const merged: Record<string, unknown> = { ...DEFAULTS };
  const sources: Record<string, string> = Object.fromEntries(
    Object.keys(DEFAULTS).map((key) => [key, 'default']),
  );
  for (const [name, layer] of layers) {
    for (const [key, value] of Object.entries(layer)) {
      if (value !== undefined) {
        merged[key] = value;
        sources[key] = name;
      }
    }
  }
  return { settings: settingsSchema.parse(merged), sources };
}

const fileLayer = layerSchema.parse(
  JSON.parse(await readFile(join(import.meta.dirname, 'config.json'), 'utf8')),
);
const dotEnv = parseEnv(await readFile(join(import.meta.dirname, 'sample.env'), 'utf8'));

const scenarios: Array<[string, Array<[string, Layer]>]> = [
  ['defaults only', []],
  ['+ config file', [['file', fileLayer]]],
  [
    '+ .env file',
    [
      ['file', fileLayer],
      ['.env', envLayer(dotEnv)],
    ],
  ],
  [
    '+ process env',
    [
      ['file', fileLayer],
      ['.env', envLayer(dotEnv)],
      ['env', envLayer({ APP_PORT: '9100' })],
    ],
  ],
  [
    '+ flags',
    [
      ['file', fileLayer],
      ['.env', envLayer(dotEnv)],
      ['env', envLayer({ APP_PORT: '9100' })],
      ['flag', flagLayer(['--port', '9200', '--log-level=error'])],
    ],
  ],
];

for (const [title, layers] of scenarios) {
  const { settings, sources } = merge(layers);
  console.log(`${title}:`, settings);
  console.log('  from  :', sources);
}

for (const bad of [() => flagLayer(['--prot', '1']), () => flagLayer(['--port', 'abc'])]) {
  try {
    bad();
  } catch (err) {
    console.log(
      'rejected:',
      err instanceof z.ZodError ? err.issues[0]?.message : (err as Error).message,
    );
  }
}
