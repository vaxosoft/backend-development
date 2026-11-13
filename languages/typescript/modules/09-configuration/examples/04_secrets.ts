// Secrets basics: keep them out of logs by type, read them from files (the *_FILE convention).
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { inspect } from 'node:util';

/** Wraps a string so that printing, JSON.stringify and console.log never reveal it. */
class Secret {
  readonly #value: string;

  constructor(value: string) {
    this.#value = value;
  }

  reveal(): string {
    return this.#value;
  }

  toString(): string {
    return '[REDACTED]';
  }

  toJSON(): string {
    return '[REDACTED]';
  }

  [inspect.custom](): string {
    return 'Secret([REDACTED])';
  }
}

type Env = Record<string, string | undefined>;

/** `NAME_FILE=/run/secrets/x` wins over `NAME=value`; both are optional. */
async function readSecret(env: Env, name: string): Promise<Secret | undefined> {
  const file = env[`${name}_FILE`];
  if (file !== undefined && file !== '') {
    return new Secret((await readFile(file, 'utf8')).trim());
  }
  const value = env[name];
  return value === undefined || value === '' ? undefined : new Secret(value);
}

const dir = await mkdtemp(join(tmpdir(), 'm09-'));
try {
  const passwordFile = join(dir, 'db_password');
  await writeFile(passwordFile, 's3cr3t-from-file\n');

  const dbPassword = await readSecret({ DB_PASSWORD_FILE: passwordFile }, 'DB_PASSWORD');
  const apiKey = await readSecret({ API_KEY: 'key-123' }, 'API_KEY');
  const missing = await readSecret({}, 'TOKEN');

  const config = { port: 8080, dbPassword, apiKey };

  console.log('console.log  :', config);
  console.log('JSON         :', JSON.stringify(config));
  console.log('template     :', `password=${dbPassword}`);
  console.log('missing      :', missing);
  console.log('length only  :', dbPassword?.reveal().length, 'characters read from the file');
} finally {
  await rm(dir, { recursive: true, force: true });
}
