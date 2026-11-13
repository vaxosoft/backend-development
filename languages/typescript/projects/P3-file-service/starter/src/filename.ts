/**
 * Turns a client-supplied file name into a safe display name.
 *
 * The result is never used as a path. It has no path separators, no `..`, no
 * control characters and no double quotes, is at most 255 characters long, and
 * is never empty (`file` is the fallback).
 */
export function sanitizeFilename(_raw: string | undefined): string {
  throw new Error('TODO');
}
