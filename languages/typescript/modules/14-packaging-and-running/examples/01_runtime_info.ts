// What the runtime tells you about how it was started: version, flags and production settings.
import { getHeapStatistics } from 'node:v8';

const MIB = 1024 * 1024;
const major = Number(process.versions.node.split('.')[0]);

console.log('node >= 24 (required by "engines"):', major >= 24);
console.log('runs .ts directly (type stripping):', process.features.typescript !== false);
console.log('execArgv (flags for node itself):', process.execArgv);
console.log('argv tail (arguments for the program):', process.argv.slice(2));
console.log('NODE_ENV:', process.env.NODE_ENV ?? '(unset)');
console.log('NODE_OPTIONS:', process.env.NODE_OPTIONS ?? '(unset)');
console.log('source maps enabled:', process.sourceMapsEnabled);
console.log('heap limit >= 512 MiB:', getHeapStatistics().heap_size_limit >= 512 * MIB);

// Libraries and frameworks switch behaviour on NODE_ENV; anything but "production" is a dev build.
const isProduction = process.env.NODE_ENV === 'production';
console.log(
  isProduction ? 'production mode: terse logs, no stack traces in responses' : 'development mode',
);
