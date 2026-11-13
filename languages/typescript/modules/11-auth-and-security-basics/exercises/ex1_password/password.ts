/**
 * Password hashing with scrypt.
 *
 * Stored format: `scrypt$<cost>$<salt>$<hash>` where `<cost>` is the scrypt `N` parameter in decimal and
 * salt (16 random bytes) and hash (32 bytes) are base64url.
 */

/** Hashes `password` with a fresh random salt. `cost` (scrypt `N`) defaults to 16384. */
export async function hashPassword(_password: string, _cost?: number): Promise<string> {
  throw new Error('TODO');
}

/**
 * True only when `password` matches `stored`. Compare in constant time.
 * Anything that is not a well-formed stored hash (wrong scheme, missing parts, non-numeric cost,
 * truncated hash...) yields `false`; this function never throws for bad input.
 * Passwords are compared after Unicode NFKC normalisation, so `"e\u0301"` equals `"\u00e9"`.
 */
export async function verifyPassword(_password: string, _stored: string): Promise<boolean> {
  throw new Error('TODO');
}

/** True when `stored` is malformed or was made with a cost lower than `targetCost`. */
export function needsRehash(_stored: string, _targetCost: number): boolean {
  throw new Error('TODO');
}
