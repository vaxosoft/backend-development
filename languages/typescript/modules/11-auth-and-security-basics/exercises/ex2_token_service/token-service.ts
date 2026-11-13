export type Role = 'admin' | 'member';

export interface Claims {
  sub: number;
  role: Role;
}

export type VerifyResult =
  | { ok: true; claims: Claims }
  | { ok: false; reason: 'expired' | 'invalid' };

export interface TokenService {
  issue(claims: Claims): Promise<string>;
  verify(token: string): Promise<VerifyResult>;
}

export interface TokenServiceOptions {
  /** HMAC secret; must be at least 32 bytes long (`createTokenService` throws a `RangeError` otherwise). */
  secret: string;
  /** Defaults to `task-tracker`. */
  issuer?: string;
  /** Token lifetime. Defaults to 3600. */
  ttlSeconds?: number;
  /** Clock used for `iat`, `exp` and verification. Defaults to `() => new Date()`. */
  now?: () => Date;
}

/**
 * A JWT (HS256) service.
 *
 * - `issue` signs a token with `sub` (the user id as a string), a `role` claim, the issuer, `iat` and `exp`.
 * - `verify` never throws. It accepts only HS256 tokens signed with the secret and issued by the
 *   issuer, then checks the claims: `sub` must be an integer string and `role` one of the known roles.
 *   An expired token yields `reason: 'expired'`; every other problem (garbage, tampering, wrong secret,
 *   `alg: none`, another algorithm, wrong issuer, unknown role...) yields `reason: 'invalid'`.
 */
export function createTokenService(_options: TokenServiceOptions): TokenService {
  throw new Error('TODO');
}
