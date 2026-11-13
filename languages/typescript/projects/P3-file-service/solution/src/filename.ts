/**
 * Turns a client-supplied file name into a safe display name.
 *
 * The result is never used as a path. It has no path separators, no `..`, no
 * control characters and no double quotes, is at most 255 characters long, and
 * is never empty (`file` is the fallback).
 */
export function sanitizeFilename(raw: string | undefined): string {
  const lastSegment = (raw ?? '').split(/[\\/]/).pop() ?? '';
  const name = [...lastSegment]
    .filter((char) => {
      const code = char.codePointAt(0) ?? 0;
      return code >= 0x20 && code !== 0x7f && char !== '"';
    })
    .join('')
    .replace(/\.{2,}/g, '.')
    .trim()
    .slice(0, 255);
  return /^\.*$/.test(name) ? 'file' : name;
}
