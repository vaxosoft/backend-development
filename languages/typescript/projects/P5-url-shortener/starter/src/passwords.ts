/**
 * TODO(P5): hash with scrypt and a fresh random salt per password.
 *
 * Return a self-describing string such as `scrypt$<cost>$<salt hex>$<hash hex>` so the cost can
 * change later without breaking old hashes. Use the callback API wrapped in a Promise (or
 * `util.promisify`): `scryptSync` would block the event loop.
 */
export async function hashPassword(_password: string, _cost: number): Promise<string> {
  throw new Error('TODO');
}

/**
 * TODO(P5): recompute the hash with the stored salt and cost and compare in constant time
 * (`timingSafeEqual` needs buffers of equal length). A malformed stored value is simply `false`.
 */
export async function verifyPassword(_password: string, _stored: string): Promise<boolean> {
  throw new Error('TODO');
}
