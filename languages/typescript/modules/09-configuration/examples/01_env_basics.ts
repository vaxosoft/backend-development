// Environment variables are strings (or undefined). Convert them once, at the edge, and fail loudly.
const env: Record<string, string | undefined> = {
  PORT: '8080',
  DEBUG: 'false',
  TIMEOUT_MS: '2.5s',
  TAGS: 'docs, p1 ,,urgent',
};

console.log('naive boolean :', Boolean(env.DEBUG), '<- the string "false" is truthy');
console.log('naive number  :', Number(env.MISSING), '<- Number(undefined) is NaN');
console.log('naive number  :', Number(''), '<- Number("") is 0');
console.log('parseInt      :', Number.parseInt('8080abc', 10), '<- silently accepts trailing junk');

function parseBool(name: string, raw: string | undefined, fallback: boolean): boolean {
  if (raw === undefined || raw === '') {
    return fallback;
  }
  const value = raw.trim().toLowerCase();
  if (['1', 'true', 'yes', 'on'].includes(value)) {
    return true;
  }
  if (['0', 'false', 'no', 'off'].includes(value)) {
    return false;
  }
  throw new Error(`${name}: expected a boolean, got "${raw}"`);
}

function parseIntInRange(
  name: string,
  raw: string | undefined,
  fallback: number,
  min: number,
  max: number,
): number {
  if (raw === undefined || raw === '') {
    return fallback;
  }
  const value = Number(raw);
  if (!Number.isInteger(value) || value < min || value > max) {
    throw new Error(`${name}: expected an integer ${min}-${max}, got "${raw}"`);
  }
  return value;
}

function parseList(raw: string | undefined): string[] {
  return (raw ?? '')
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item !== '');
}

console.log('PORT          :', parseIntInRange('PORT', env.PORT, 3000, 1, 65535));
console.log('DEBUG         :', parseBool('DEBUG', env.DEBUG, false));
console.log('LOG_JSON      :', parseBool('LOG_JSON', env.LOG_JSON, true), '(default)');
console.log('TAGS          :', parseList(env.TAGS));

try {
  parseIntInRange('TIMEOUT_MS', env.TIMEOUT_MS, 1000, 1, 60000);
} catch (err) {
  console.log('fail fast     :', (err as Error).message);
}
