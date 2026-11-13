import { greeting } from './greeting.ts';

const version = process.env.APP_VERSION ?? 'dev';
console.log(greeting('packaging', version));
console.log(
  'running as:',
  import.meta.url.endsWith('.ts') ? 'TypeScript source' : 'compiled JavaScript',
);
