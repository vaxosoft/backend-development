import { jwtVerify, SignJWT } from 'jose';

const key = (secret: string) => new TextEncoder().encode(secret);

export function signToken(userId: number, secret: string): Promise<string> {
  return new SignJWT()
    .setProtectedHeader({ alg: 'HS256' })
    .setSubject(String(userId))
    .setIssuedAt()
    .setExpirationTime('1h')
    .sign(key(secret));
}

export async function verifyToken(token: string, secret: string): Promise<number | null> {
  try {
    const { payload } = await jwtVerify(token, key(secret), { algorithms: ['HS256'] });
    const userId = Number(payload.sub);
    return Number.isInteger(userId) ? userId : null;
  } catch {
    return null;
  }
}
