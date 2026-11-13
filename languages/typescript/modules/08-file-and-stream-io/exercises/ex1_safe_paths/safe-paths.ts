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
export function resolveInside(_root: string, _untrusted: string): string {
  throw new Error('TODO');
}

/**
 * Turns a client-supplied file name into a harmless single path segment:
 * - keep only what follows the last `/` or `\`
 * - remove control characters (code points 0-31 and 127) and trim surrounding whitespace
 * - an empty result, `.` or `..` becomes `unnamed`
 * - at most 100 characters
 */
export function sanitizeFilename(_name: string): string {
  throw new Error('TODO');
}
