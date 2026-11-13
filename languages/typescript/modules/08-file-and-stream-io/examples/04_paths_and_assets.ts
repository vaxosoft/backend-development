// Paths, untrusted file names and bundled assets.
import { readFile } from 'node:fs/promises';
import { basename, extname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import defaults from './assets/defaults.json' with { type: 'json' };

console.log(join('/srv/uploads', 'a', '..', 'b', 'file.txt'));
console.log(resolve('/srv/uploads', '/etc/passwd'));
console.log(basename('/srv/uploads/report.final.pdf'), extname('report.final.pdf'));
console.log(relative('/srv/uploads', '/srv/uploads/2030/report.pdf'));

/** Resolves `untrusted` inside `root` or throws if the result would escape it. */
function resolveInside(root: string, untrusted: string): string {
  const base = resolve(root);
  const target = resolve(base, untrusted);
  const rel = relative(base, target);
  if (rel === '' || rel.startsWith(`..${sep}`) || rel === '..' || isAbsolute(rel)) {
    throw new Error(`path escapes ${base}: ${untrusted}`);
  }
  return target;
}

for (const name of [
  'report.pdf',
  'sub/dir/report.pdf',
  '../../etc/passwd',
  '/etc/passwd',
  'a/../../b',
]) {
  try {
    console.log('ok     ', name, '->', resolveInside('/srv/uploads', name));
  } catch (err) {
    console.log('refused', (err as Error).message);
  }
}

// Assets: resolve relative to this module, never relative to the process's current directory.
const banner = await readFile(join(import.meta.dirname, 'assets', 'banner.txt'), 'utf8');
console.log(banner.split('\n')[0]);

// Small static data can be embedded at load time with a JSON import attribute.
console.log(defaults.defaultTags, defaults.maxExportRows);
