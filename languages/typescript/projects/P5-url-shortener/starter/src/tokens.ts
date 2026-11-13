/**
 * TODO(P5): sign an HS256 JWT with `jose`: subject = user id as a string, `iat` now,
 * expiry in one hour. The secret is a string, so turn it into bytes with `TextEncoder`.
 */
export async function signToken(_userId: number, _secret: string): Promise<string> {
  throw new Error('TODO');
}

/**
 * TODO(P5): verify signature and expiry and return the user id, or null for ANY problem
 * (malformed, expired, wrong signature, unsigned `alg: none`, ...). Pin the accepted algorithm to
 * HS256: never let the token choose how it is verified. This function must not throw.
 */
export async function verifyToken(_token: string, _secret: string): Promise<number | null> {
  throw new Error('TODO');
}
