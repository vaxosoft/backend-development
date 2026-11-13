import { isAbsolute, relative, resolve, sep } from 'node:path';

export class PathEscapeError extends Error {
  constructor(root: string, untrusted: string) {
    super(`path escapes ${root}: ${JSON.stringify(untrusted)}`);
    this.name = 'PathEscapeError';
  }
}

/**
 * Resolves `untrusted` (a user-supplied relative path) inside `root` and returns the absolute path.
 * Throws `PathEscapeError` when the result is outside `root` (`..` segments, absolute paths,
 * a sibling directory that merely shares a prefix such as /srv/up vs /srv/up-evil) or is `root` itself.
 */
export function resolveInside(root: string, untrusted: string): string {
  const base = resolve(root);
  const target = resolve(base, untrusted);
  const rel = relative(base, target);
  if (rel === '' || rel === '..' || rel.startsWith(`..${sep}`) || isAbsolute(rel)) {
    throw new PathEscapeError(base, untrusted);
  }
  return target;
}

/**
 * Turns a client-supplied file name into a harmless single path segment:
 * - keep only what follows the last `/` or `\`
 * - remove control characters (code points 0-31 and 127) and trim surrounding whitespace
 * - an empty result, `.` or `..` becomes `unnamed`
 * - at most 100 characters
 */
export function sanitizeFilename(name: string): string {
  const lastSegment = name.slice(Math.max(name.lastIndexOf('/'), name.lastIndexOf('\\')) + 1);
  let cleaned = '';
  for (const char of lastSegment) {
    const code = char.codePointAt(0) ?? 0;
    if (code > 31 && code !== 127) {
      cleaned += char;
    }
  }
  cleaned = cleaned.trim();
  if (cleaned === '' || cleaned === '.' || cleaned === '..') {
    return 'unnamed';
  }
  return cleaned.slice(0, 100);
}
