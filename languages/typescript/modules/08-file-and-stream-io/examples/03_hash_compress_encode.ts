// Hashing, compression and encodings, all streaming where the data can be large.
import { createHash } from 'node:crypto';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { createGunzip, createGzip, gunzipSync, gzipSync } from 'node:zlib';

async function sha256Of(source: AsyncIterable<Buffer | string>): Promise<string> {
  const hash = createHash('sha256');
  for await (const chunk of source) {
    hash.update(chunk);
  }
  return hash.digest('hex');
}

console.log('sha256("hello") =', createHash('sha256').update('hello').digest('hex'));
console.log('streamed        =', await sha256Of(Readable.from(['hel', 'lo'])));

// Encodings live on Buffer.
const bytes = Buffer.from('héllo?>', 'utf8');
console.log('hex       ', bytes.toString('hex'));
console.log('base64    ', bytes.toString('base64'));
console.log('base64url ', bytes.toString('base64url'));
console.log('decoded   ', Buffer.from(bytes.toString('base64'), 'base64').toString('utf8'));

// Small payloads: the synchronous helpers are fine.
const text = 'task '.repeat(1000);
const small = gzipSync(text);
console.log('gzip      ', text.length, '->', small.length, gunzipSync(small).toString() === text);

// Large payloads: gzip then gunzip as streams, hashing what comes out.
const chunks: Buffer[] = [];
await pipeline(
  Readable.from([text, text]),
  createGzip(),
  createGunzip(),
  async (source: AsyncIterable<Buffer>) => {
    for await (const chunk of source) {
      chunks.push(chunk);
    }
  },
);
console.log('roundtrip ', Buffer.concat(chunks).toString() === text + text);
