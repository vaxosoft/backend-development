import { randomBytes, scrypt, timingSafeEqual } from 'node:crypto';

/**
 * Password hashing with scrypt.
 *
 * Stored format: `scrypt$<cost>$<salt>$<hash>` where `<cost>` is the scrypt `N` parameter in decimal and
 * salt (16 random bytes) and hash (32 bytes) are base64url.
 */

const KEY_LENGTH = 32;
const DEFAULT_COST = 16384;

function derive(password: string, salt: Buffer, cost: number): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    scrypt(password.normalize('NFKC'), salt, KEY_LENGTH, { N: cost }, (err, key) => {
      if (err) reject(err);
      else resolve(key);
    });
  });
}

/** Hashes `password` with a fresh random salt. `cost` (scrypt `N`) defaults to 16384. */
export async function hashPassword(password: string, cost = DEFAULT_COST): Promise<string> {
  const salt = randomBytes(16);
  const key = await derive(password, salt, cost);
  return `scrypt$${cost}$${salt.toString('base64url')}$${key.toString('base64url')}`;
}

interface Parsed {
  cost: number;
  salt: Buffer;
  key: Buffer;
}

function parse(stored: string): Parsed | null {
  const parts = stored.split('$');
  if (parts.length !== 4 || parts[0] !== 'scrypt') return null;
  const [, costText = '', saltText = '', keyText = ''] = parts;
  const cost = Number(costText);
  if (!/^\d+$/.test(costText) || cost < 2 || !saltText || !keyText) return null;
  return { cost, salt: Buffer.from(saltText, 'base64url'), key: Buffer.from(keyText, 'base64url') };
}

/**
 * True only when `password` matches `stored`. Compare in constant time.
 * Anything that is not a well-formed stored hash (wrong scheme, missing parts, non-numeric cost,
 * truncated hash...) yields `false`; this function never throws for bad input.
 * Passwords are compared after Unicode NFKC normalisation, so `"e\u0301"` equals `"\u00e9"`.
 */
export async function verifyPassword(password: string, stored: string): Promise<boolean> {
  const parsed = parse(stored);
  if (!parsed) return false;
  try {
    const actual = await derive(password, parsed.salt, parsed.cost);
    return actual.length === parsed.key.length && timingSafeEqual(actual, parsed.key);
  } catch {
    return false;
  }
}

/** True when `stored` is malformed or was made with a cost lower than `targetCost`. */
export function needsRehash(stored: string, targetCost: number): boolean {
  const parsed = parse(stored);
  return parsed === null || parsed.cost < targetCost;
}
